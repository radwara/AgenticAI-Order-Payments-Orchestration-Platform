package server_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/server"
	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/store"
	inventoryv1 "github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/inventorypb/v1"
)

// fakeStore holds one row; err, when set, fails every call.
type fakeStore struct {
	row *store.Stock
	err error
}

func (f *fakeStore) SetStock(_ context.Context, sku, location string, onHand int64) (store.Stock, error) {
	if f.err != nil {
		return store.Stock{}, f.err
	}
	f.row = &store.Stock{SKU: sku, Location: location, OnHand: onHand, Version: 1,
		UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	return *f.row, nil
}

func (f *fakeStore) GetStock(context.Context, string, string) (store.Stock, error) {
	switch {
	case f.err != nil:
		return store.Stock{}, f.err
	case f.row == nil:
		return store.Stock{}, store.ErrNotFound
	}
	return *f.row, nil
}

func TestSetStockThenGetStock_RoundTrips(t *testing.T) {
	ctx := context.Background()
	srv := server.New(&fakeStore{})
	if _, err := srv.SetStock(ctx, &inventoryv1.SetStockRequest{Sku: "A", Location: "wh1", OnHand: 10}); err != nil {
		t.Fatalf("SetStock: %v", err)
	}
	r, err := srv.GetStock(ctx, &inventoryv1.GetStockRequest{Sku: "A", Location: "wh1"})
	if err != nil {
		t.Fatalf("GetStock: %v", err)
	}
	s := r.GetStock()
	if s.GetSku() != "A" || s.GetLocation() != "wh1" || s.GetOnHand() != 10 || s.GetAvailable() != 10 ||
		s.GetVersion() != 1 || s.GetUpdatedAt() != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected stock: %v", s)
	}
}

func TestGetStock_UnknownSkuIsNotFound(t *testing.T) {
	_, err := server.New(&fakeStore{}).GetStock(context.Background(), &inventoryv1.GetStockRequest{Sku: "NOPE", Location: "wh1"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestInvalidInputIsInvalidArgument(t *testing.T) {
	srv := server.New(&fakeStore{})
	for name, key := range map[string]string{"empty": "", "nul": "A\x00B", "too_long": strings.Repeat("x", server.MaxKeyLen+1),
		"edge_whitespace": "A ", "invalid_utf8": "\xff"} {
		_, setErr := srv.SetStock(context.Background(), &inventoryv1.SetStockRequest{Sku: key, Location: "wh1"})
		_, getErr := srv.GetStock(context.Background(), &inventoryv1.GetStockRequest{Sku: "A", Location: key})
		if status.Code(setErr) != codes.InvalidArgument || status.Code(getErr) != codes.InvalidArgument {
			t.Errorf("%s: want InvalidArgument, got %v / %v", name, setErr, getErr)
		}
	}
	_, err := srv.SetStock(context.Background(), &inventoryv1.SetStockRequest{Sku: "A", Location: "wh1", OnHand: -1})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("negative on_hand: got %v", err)
	}
}

func TestStoreErrorsMapToGRPCCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want codes.Code
	}{
		"below_reserved": {store.ErrInvalidStock, codes.InvalidArgument},
		"canceled":       {context.Canceled, codes.Canceled},
		"deadline":       {context.DeadlineExceeded, codes.DeadlineExceeded},
		"db_failure":     {errors.New("connection refused to 10.0.0.1"), codes.Internal},
	} {
		_, err := server.New(&fakeStore{err: tc.err}).SetStock(context.Background(),
			&inventoryv1.SetStockRequest{Sku: "A", Location: "wh1", OnHand: 1})
		if status.Code(err) != tc.want || strings.Contains(status.Convert(err).Message(), "10.0.0.1") {
			t.Errorf("%s: want %v without internals, got %v", name, tc.want, err)
		}
	}
}
