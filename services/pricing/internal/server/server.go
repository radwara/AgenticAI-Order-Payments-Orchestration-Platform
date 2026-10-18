// Package server adapts the quote service to gRPC and exposes HTTP health probes.
package server

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/radwara/agentic-order-platform/services/pricing/gen/pricingv1"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/quote"
)

// GRPC implements pricing.v1.Pricing.
type GRPC struct {
	pb.UnimplementedPricingServer
	svc *quote.Service
}

// NewGRPC returns a gRPC handler backed by svc.
func NewGRPC(svc *quote.Service) *GRPC { return &GRPC{svc: svc} }

// Quote implements pricing.v1.Pricing/Quote. It maps quote.ErrInvalid to
// INVALID_ARGUMENT and quote.ErrNotFound to NOT_FOUND. It is idempotent.
func (g *GRPC) Quote(_ context.Context, in *pb.QuoteRequest) (*pb.QuoteResponse, error) {
	req := quote.Request{PriceListVersion: in.GetPriceListVersion(), PromoVersion: in.GetPromotionRuleVersion()}
	for _, it := range in.GetItems() {
		item := quote.Item{SKU: it.GetSku(), Quantity: it.GetQuantity(), Currency: it.GetCurrency()}
		if it.UnitPriceMinor != nil { // presence matters: 0 is a real assertion
			price := it.GetUnitPriceMinor()
			item.UnitPriceMinor = &price
		}
		req.Items = append(req.Items, item)
	}
	res, err := g.svc.Quote(req)
	switch {
	case errors.Is(err, quote.ErrNotFound):
		return nil, status.Error(codes.NotFound, err.Error())
	case errors.Is(err, quote.ErrInvalid):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	case err != nil:
		return nil, status.Error(codes.Internal, "quote failed")
	}
	return res, nil
}

// HealthMux serves /healthz (liveness) and /readyz (readiness). Price data is
// loaded before the server starts, so both report 200 once the mux is serving.
func HealthMux() *http.ServeMux {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	mux.HandleFunc("/healthz", ok)
	mux.HandleFunc("/readyz", ok)
	return mux
}
