// Package store is the pgx data access layer for the stock table.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pgCheckViolation is the SQLSTATE for a violated CHECK constraint.
const pgCheckViolation = "23514"

// QueryTimeout bounds each query on top of the caller's deadline.
const QueryTimeout = 5 * time.Second

var (
	// ErrNotFound: no stock row for the SKU and location.
	ErrNotFound = errors.New("stock not found")
	// ErrInvalidStock: on_hand negative or below reserved.
	ErrInvalidStock = errors.New("invalid stock level")
)

// Stock is one row of the stock table.
type Stock struct {
	SKU       string
	Location  string
	OnHand    int64
	Reserved  int64
	Version   int64
	UpdatedAt time.Time
}

// Store reads and writes stock rows.
type Store struct{ db *pgxpool.Pool }

// New returns a Store backed by db.
func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

const (
	setStockSQL = `INSERT INTO stock (sku, location, on_hand)
VALUES ($1, $2, $3)
ON CONFLICT (sku, location) DO UPDATE
SET on_hand = EXCLUDED.on_hand, version = stock.version + 1, updated_at = now()
RETURNING sku, location, on_hand, reserved, version, updated_at`

	getStockSQL = `SELECT sku, location, on_hand, reserved, version, updated_at
FROM stock WHERE sku = $1 AND location = $2`
)

// SetStock upserts on_hand for the SKU and location and returns the new row.
// Reserved is untouched. ErrInvalidStock if the value is negative or below
// reserved. Idempotent for the quantity; the version still bumps.
func (s *Store) SetStock(ctx context.Context, sku, location string, onHand int64) (Stock, error) {
	st, err := s.one(ctx, setStockSQL, sku, location, onHand)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgCheckViolation {
		err = ErrInvalidStock
	}
	return st, wrap("set stock", err)
}

// GetStock returns the row for the SKU and location, or ErrNotFound.
func (s *Store) GetStock(ctx context.Context, sku, location string) (Stock, error) {
	st, err := s.one(ctx, getStockSQL, sku, location)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return st, wrap("get stock", err)
}

func (s *Store) one(ctx context.Context, sql string, args ...any) (Stock, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeout)
	defer cancel()
	var st Stock
	err := s.db.QueryRow(ctx, sql, args...).Scan(&st.SKU, &st.Location, &st.OnHand, &st.Reserved, &st.Version, &st.UpdatedAt)
	return st, err
}

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
