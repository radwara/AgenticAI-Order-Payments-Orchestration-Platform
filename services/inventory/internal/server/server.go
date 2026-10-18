// Package server implements the inventory gRPC API.
package server

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/store"
	inventoryv1 "github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/inventorypb/v1"
)

// MaxKeyLen is the maximum length in bytes of a sku or location.
const MaxKeyLen = 128

// StockStore is the persistence the server depends on.
type StockStore interface {
	SetStock(ctx context.Context, sku, location string, onHand int64) (store.Stock, error)
	GetStock(ctx context.Context, sku, location string) (store.Stock, error)
}

// Server implements inventoryv1.InventoryServiceServer.
type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	store StockStore
}

// New returns a Server backed by s.
func New(s StockStore) *Server { return &Server{store: s} }

// SetStock sets on_hand for a SKU and location. Returns INVALID_ARGUMENT for an
// invalid sku/location, a negative quantity, or a quantity below reserved.
func (s *Server) SetStock(ctx context.Context, req *inventoryv1.SetStockRequest) (*inventoryv1.StockLevel, error) {
	if err := validateKey(req.GetSku(), req.GetLocation()); err != nil {
		return nil, err
	}
	if req.GetOnHand() < 0 {
		return nil, status.Error(codes.InvalidArgument, "on_hand must not be negative")
	}
	st, err := s.store.SetStock(ctx, req.GetSku(), req.GetLocation(), req.GetOnHand())
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return toProto(st), nil
}

// GetStock returns the stock level, or NOT_FOUND for an unknown SKU/location.
func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.StockLevel, error) {
	if err := validateKey(req.GetSku(), req.GetLocation()); err != nil {
		return nil, err
	}
	st, err := s.store.GetStock(ctx, req.GetSku(), req.GetLocation())
	if err != nil {
		return nil, toStatus(ctx, err)
	}
	return toProto(st), nil
}

// validateKey requires 1..MaxKeyLen bytes of valid UTF-8 without NUL, and no
// surrounding whitespace (so "A " cannot silently become a second key).
func validateKey(sku, location string) error {
	for _, v := range []string{sku, location} {
		if v == "" || len(v) > MaxKeyLen || !utf8.ValidString(v) ||
			strings.ContainsRune(v, 0) || strings.TrimSpace(v) != v {
			return status.Errorf(codes.InvalidArgument,
				"sku and location must be 1-%d chars of valid UTF-8, no NUL or edge whitespace", MaxKeyLen)
		}
	}
	return nil
}

func toStatus(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, "stock not found")
	case errors.Is(err, store.ErrInvalidStock):
		return status.Error(codes.InvalidArgument, "on_hand must be non-negative and not below reserved")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	default:
		slog.ErrorContext(ctx, "inventory request failed", "error", err)
		return status.Error(codes.Internal, "internal error")
	}
}

func toProto(st store.Stock) *inventoryv1.StockLevel {
	return &inventoryv1.StockLevel{
		Sku:       st.SKU,
		Location:  st.Location,
		OnHand:    st.OnHand,
		Reserved:  st.Reserved,
		Available: st.OnHand - st.Reserved,
		Version:   st.Version,
		UpdatedAt: st.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
