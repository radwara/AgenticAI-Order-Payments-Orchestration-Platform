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

const startTimeout = 90 * time.Second

// setup starts a clean Postgres and returns a pool plus the migrator. No Docker:
// skip locally, fail when CI=true so CI never silently skips.
func setup(t *testing.T) (*pgxpool.Pool, *migrate.Migrate) {
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
	m, err := migrate.New("file://../../migrations", "pgx5"+dsn[len("postgres"):])
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, m
}

func migrated(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, m := setup(t)
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return pool
}

func TestMigration_UpThenDownSucceedsOnCleanDatabase(t *testing.T) {
	pool, m := setup(t)
	exists := func() (ok bool) {
		err := pool.QueryRow(context.Background(), `SELECT to_regclass('public.stock') IS NOT NULL`).Scan(&ok)
		if err != nil {
			t.Fatalf("table check: %v", err)
		}
		return ok
	}
	if err := m.Up(); err != nil || !exists() {
		t.Fatalf("up failed or stock table missing: err=%v", err)
	}
	if err := m.Down(); err != nil || exists() {
		t.Fatalf("down failed or stock table still present: err=%v", err)
	}
}

func TestStockConstraint_RejectsInvalidRows(t *testing.T) {
	pool := migrated(t)
	for name, v := range map[string][2]int64{
		"reserved_greater_than_on_hand": {5, 6}, "negative_reserved": {5, -1}, "negative_on_hand": {-1, 0},
	} {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO stock (sku, location, on_hand, reserved) VALUES ($1, 'wh1', $2, $3)`, name, v[0], v[1])
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("%s: want check violation 23514, got %v", name, err)
		}
	}
}

func TestStore_SetThenGetRoundTrips(t *testing.T) {
	ctx := context.Background()
	s := store.New(migrated(t))

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

func TestStore_ErrorsAreMapped(t *testing.T) {
	ctx := context.Background()
	pool := migrated(t)
	s := store.New(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO stock (sku, location, on_hand, reserved) VALUES ('R', 'wh1', 10, 8)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.GetStock(ctx, "NOPE", "wh1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown sku: want ErrNotFound, got %v", err)
	}
	if _, err := s.SetStock(ctx, "R", "wh1", 7); !errors.Is(err, store.ErrInvalidStock) {
		t.Errorf("below reserved: want ErrInvalidStock, got %v", err)
	}
	if _, err := s.SetStock(ctx, "NEG", "wh1", -1); !errors.Is(err, store.ErrInvalidStock) {
		t.Errorf("negative: want ErrInvalidStock, got %v", err)
	}
}
