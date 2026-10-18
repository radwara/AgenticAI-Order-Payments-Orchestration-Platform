package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers pgx5:// driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // registers file:// source
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/store"
)

const (
	migrationsURL  = "file://../../migrations"
	checkViolation = "23514"
	startTimeout   = 90 * time.Second
)

// startPostgres returns a clean Postgres DSN. No Docker: skip locally, fail when CI=true.
func startPostgres(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("inventory"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(startTimeout)))
	if err != nil {
		if os.Getenv("CI") == "true" {
			t.Fatalf("postgres container (required in CI): %v", err)
		}
		t.Skipf("Docker unavailable, skipping Postgres integration test: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}

func newPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newMigrate(t *testing.T, dsn string) *migrate.Migrate {
	t.Helper()
	m, err := migrate.New(migrationsURL, "pgx5"+dsn[len("postgres"):])
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	return m
}

func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := startPostgres(t)
	if err := newMigrate(t, dsn).Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return newPool(t, dsn)
}

func stockTableExists(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(), `SELECT to_regclass('public.stock') IS NOT NULL`).Scan(&exists)
	if err != nil {
		t.Fatalf("table check: %v", err)
	}
	return exists
}

func TestMigration_UpThenDownSucceedsOnCleanDatabase(t *testing.T) {
	dsn := startPostgres(t)
	m, pool := newMigrate(t, dsn), newPool(t, dsn)

	if err := m.Up(); err != nil || !stockTableExists(t, pool) {
		t.Fatalf("up failed or stock table missing: err=%v", err)
	}
	if err := m.Down(); err != nil || stockTableExists(t, pool) {
		t.Fatalf("down failed or stock table still present: err=%v", err)
	}
}

func TestStockConstraint_RejectsInvalidRows(t *testing.T) {
	pool := migratedPool(t)
	tests := []struct {
		name             string
		onHand, reserved int64
	}{
		{"reserved_greater_than_on_hand", 5, 6},
		{"negative_reserved", 5, -1},
		{"negative_on_hand", -1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(context.Background(),
				`INSERT INTO stock (sku, location, on_hand, reserved) VALUES ($1, 'wh1', $2, $3)`,
				tc.name, tc.onHand, tc.reserved)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != checkViolation {
				t.Fatalf("want check violation %s, got %v", checkViolation, err)
			}
		})
	}
}

func TestStore_SetThenGetRoundTrips(t *testing.T) {
	ctx := context.Background()
	s := store.New(migratedPool(t))

	set, err := s.SetStock(ctx, "SKU-1", "wh1", 10)
	if err != nil {
		t.Fatalf("SetStock: %v", err)
	}
	got, err := s.GetStock(ctx, "SKU-1", "wh1")
	if err != nil || got != set || got.OnHand != 10 || got.Reserved != 0 || got.Version != 1 {
		t.Fatalf("round trip mismatch: set=%+v got=%+v err=%v", set, got, err)
	}
	if again, err := s.SetStock(ctx, "SKU-1", "wh1", 12); err != nil || again.OnHand != 12 || again.Version != 2 {
		t.Fatalf("update want on_hand 12 version 2, got %+v err=%v", again, err)
	}
}

func TestStore_GetUnknownSkuIsNotFound(t *testing.T) {
	_, err := store.New(migratedPool(t)).GetStock(context.Background(), "NOPE", "wh1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestStore_SetBelowReservedOrNegativeIsInvalid(t *testing.T) {
	ctx := context.Background()
	pool := migratedPool(t)
	s := store.New(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO stock (sku, location, on_hand, reserved) VALUES ('R', 'wh1', 10, 8)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.SetStock(ctx, "R", "wh1", 7); !errors.Is(err, store.ErrInvalidStock) {
		t.Fatalf("below reserved: want ErrInvalidStock, got %v", err)
	}
	if _, err := s.SetStock(ctx, "NEG", "wh1", -1); !errors.Is(err, store.ErrInvalidStock) {
		t.Fatalf("negative: want ErrInvalidStock, got %v", err)
	}
}
