// Command pricing serves the pricing gRPC API and HTTP health probes.
// Config from env: PRICING_GRPC_ADDR (:9090), PRICING_HTTP_ADDR (:8080),
// PRICING_DATA_DIR (testdata; v1 loads versioned JSON files from disk).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	pb "github.com/radwara/agentic-order-platform/services/pricing/gen/pricingv1"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/catalog"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/quote"
	"github.com/radwara/agentic-order-platform/services/pricing/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("pricing failed", "err", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func run() error {
	cat, err := catalog.Load(os.DirFS(env("PRICING_DATA_DIR", "testdata")))
	if err != nil {
		return err
	}
	lis, err := net.Listen("tcp", env("PRICING_GRPC_ADDR", ":9090"))
	if err != nil {
		return err
	}
	grpcSrv := grpc.NewServer()
	pb.RegisterPricingServer(grpcSrv, server.NewGRPC(quote.NewService(cat)))
	httpSrv := &http.Server{
		Addr: env("PRICING_HTTP_ADDR", ":8080"), Handler: server.HealthMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 2)
	go func() { errc <- grpcSrv.Serve(lis) }()
	go func() { errc <- httpSrv.ListenAndServe() }()

	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	grpcSrv.GracefulStop()
	_ = httpSrv.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
