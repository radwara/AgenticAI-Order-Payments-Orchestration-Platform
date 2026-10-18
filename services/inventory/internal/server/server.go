// Package server implements the inventory gRPC API.
package server

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/store"
	inventoryv1 "github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/inventorypb/v1"
)

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
// empty sku/location, a negative quantity, or a quantity below reserved.
func (s *Server) SetStock(ctx context.Context, req *inventoryv1.SetStockRequest) (*inventoryv1.SetStockResponse, error) {
	if err := validateKey(req.GetSku(), req.GetLocation()); err != nil {
		return nil, err
	}
	if req.GetOnHand() < 0 {
		return nil, status.Error(codes.InvalidArgument, "on_hand must not be negative")
	}
	st, err := s.store.SetStock(ctx, req.GetSku(), req.GetLocation(), req.GetOnHand())
	if err != nil {
		return nil, toStatus(err)
	}
	return &inventoryv1.SetStockResponse{Stock: toProto(st)}, nil
}

// GetStock returns the stock level, or NOT_FOUND for an unknown SKU/location.
func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	if err := validateKey(req.GetSku(), req.GetLocation()); err != nil {
		return nil, err
	}
	st, err := s.store.GetStock(ctx, req.GetSku(), req.GetLocation())
	if err != nil {
		return nil, toStatus(err)
	}
	return &inventoryv1.GetStockResponse{Stock: toProto(st)}, nil
}

func validateKey(sku, location string) error {
	if sku == "" || location == "" {
		return status.Error(codes.InvalidArgument, "sku and location are required")
	}
	return nil
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, "stock not found")
	case errors.Is(err, store.ErrInvalidStock):
		return status.Error(codes.InvalidArgument, "on_hand must be non-negative and not below reserved")
	default:
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
