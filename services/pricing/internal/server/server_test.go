package server_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "github.com/radwara/agentic-order-platform/services/pricing/gen/pricingv1"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/catalog"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/quote"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/server"
)

// startClient runs the real gRPC server on a loopback listener.
func startClient(t *testing.T) pb.PricingClient {
	t.Helper()
	cat, err := catalog.Load(os.DirFS("../../testdata"))
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterPricingServer(srv, server.NewGRPC(quote.NewService(cat)))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewPricingClient(conn)
}

func request(qty int64) *pb.QuoteRequest {
	listPrice := int64(1999) // matching assertions must be accepted
	return &pb.QuoteRequest{
		Items:                []*pb.LineItem{{Sku: "SKU-A", Quantity: qty, UnitPriceMinor: &listPrice, Currency: "USD"}, {Sku: "SKU-B", Quantity: 2}},
		PriceListVersion:     "pl-usd-v1",
		PromotionRuleVersion: "promo-v1",
	}
}

func TestQuote_ThousandCallsReturnIdenticalBytes(t *testing.T) {
	client := startClient(t)
	marshal := proto.MarshalOptions{Deterministic: true}
	var first []byte
	for i := 0; i < 1000; i++ {
		resp, err := client.Quote(context.Background(), request(4))
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		b, err := marshal.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if first == nil {
			first = b
			// 7996-1000 + 2002-300
			if resp.GetTotalMinor() != 8698 || len(resp.GetQuoteHash()) != 64 {
				t.Fatalf("unexpected response: %v", resp)
			}
		} else if !bytes.Equal(first, b) {
			t.Fatalf("call %d differs from call 0", i)
		}
	}
}

func TestQuote_MapsErrorsToGRPCCodes(t *testing.T) {
	client := startClient(t)
	zero := int64(0)
	item := func(it *pb.LineItem) *pb.QuoteRequest {
		return &pb.QuoteRequest{Items: []*pb.LineItem{it}, PriceListVersion: "pl-usd-v1"}
	}
	unknownList := item(&pb.LineItem{Sku: "SKU-A", Quantity: 1})
	unknownList.PriceListVersion = "nope"
	tests := []struct {
		name string
		req  *pb.QuoteRequest
		want codes.Code
	}{
		{"unknown price list", unknownList, codes.NotFound},
		{"unknown sku", item(&pb.LineItem{Sku: "SKU-Z", Quantity: 1}), codes.NotFound},
		{"negative quantity", item(&pb.LineItem{Sku: "SKU-A", Quantity: -1}), codes.InvalidArgument},
		{"asserted zero price is a mismatch, not unset", item(&pb.LineItem{Sku: "SKU-A", Quantity: 1, UnitPriceMinor: &zero}), codes.InvalidArgument},
		{"asserted currency mismatch", item(&pb.LineItem{Sku: "SKU-A", Quantity: 1, Currency: "JPY"}), codes.InvalidArgument},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.Quote(context.Background(), tc.req)
			if got := status.Code(err); got != tc.want {
				t.Fatalf("code = %v, want %v (%v)", got, tc.want, err)
			}
		})
	}
}

func TestHealthEndpointsReturn200(t *testing.T) {
	h := server.HealthMux()
	for _, p := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200", p, rec.Code)
		}
	}
}
