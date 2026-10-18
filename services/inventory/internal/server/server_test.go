package server_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/server"
	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/store"
	inventoryv1 "github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/inventorypb/v1"
)

type fakeStore struct {
	rows map[string]store.Stock
	err  error
}

func newFake() *fakeStore { return &fakeStore{rows: map[string]store.Stock{}} }

func (f *fakeStore) SetStock(_ context.Context, sku, location string, onHand int64) (store.Stock, error) {
	if f.err != nil {
		return store.Stock{}, f.err
	}
	st := f.rows[sku+"/"+location]
	if onHand < st.Reserved {
		return store.Stock{}, store.ErrInvalidStock
	}
	st = store.Stock{SKU: sku, Location: location, OnHand: onHand, Reserved: st.Reserved,
		Version: st.Version + 1, UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	f.rows[sku+"/"+location] = st
	return st, nil
}

func (f *fakeStore) GetStock(_ context.Context, sku, location string) (store.Stock, error) {
	if f.err != nil {
		return store.Stock{}, f.err
	}
	st, ok := f.rows[sku+"/"+location]
	if !ok {
		return store.Stock{}, store.ErrNotFound
	}
	return st, nil
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("want code %v, got %v (err=%v)", want, got, err)
	}
}

func TestSetStockThenGetStock_RoundTrips(t *testing.T) {
	ctx := context.Background()
	srv := server.New(newFake())

	if _, err := srv.SetStock(ctx, &inventoryv1.SetStockRequest{Sku: "A", Location: "wh1", OnHand: 10}); err != nil {
		t.Fatalf("SetStock: %v", err)
	}
	got, err := srv.GetStock(ctx, &inventoryv1.GetStockRequest{Sku: "A", Location: "wh1"})
	if err != nil {
		t.Fatalf("GetStock: %v", err)
	}
	s := got.GetStock()
	if s.GetSku() != "A" || s.GetLocation() != "wh1" || s.GetOnHand() != 10 || s.GetAvailable() != 10 ||
		s.GetVersion() != 1 || s.GetUpdatedAt() != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected stock: %v", s)
	}
}

func TestGetStock_UnknownSkuIsNotFound(t *testing.T) {
	_, err := server.New(newFake()).GetStock(context.Background(),
		&inventoryv1.GetStockRequest{Sku: "NOPE", Location: "wh1"})
	wantCode(t, err, codes.NotFound)
}

func TestRequests_RejectBadInput(t *testing.T) {
	srv := server.New(newFake())
	ctx := context.Background()
	_, err := srv.SetStock(ctx, &inventoryv1.SetStockRequest{Location: "wh1", OnHand: 1})
	wantCode(t, err, codes.InvalidArgument)
	_, err = srv.SetStock(ctx, &inventoryv1.SetStockRequest{Sku: "A", Location: "wh1", OnHand: -1})
	wantCode(t, err, codes.InvalidArgument)
	_, err = srv.GetStock(ctx, &inventoryv1.GetStockRequest{Sku: "A"})
	wantCode(t, err, codes.InvalidArgument)
}

func TestSetStock_BelowReservedIsInvalidArgument(t *testing.T) {
	f := newFake()
	f.rows["A/wh1"] = store.Stock{SKU: "A", Location: "wh1", OnHand: 10, Reserved: 8}
	_, err := server.New(f).SetStock(context.Background(),
		&inventoryv1.SetStockRequest{Sku: "A", Location: "wh1", OnHand: 5})
	wantCode(t, err, codes.InvalidArgument)
}

func TestStoreFailureMapsToInternalWithoutLeakingDetails(t *testing.T) {
	f := newFake()
	f.err = errors.New("connection refused to 10.0.0.1")
	srv := server.New(f)

	_, err := srv.GetStock(context.Background(), &inventoryv1.GetStockRequest{Sku: "A", Location: "wh1"})
	wantCode(t, err, codes.Internal)
	if status.Convert(err).Message() != "internal error" {
		t.Fatalf("message leaks internals: %q", status.Convert(err).Message())
	}
}
