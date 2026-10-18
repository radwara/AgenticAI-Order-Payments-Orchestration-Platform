// Command inventory runs the inventory gRPC service.
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/server"
	"github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/internal/store"
	inventoryv1 "github.com/radwara/agenticai-order-payments-orchestration-platform/services/inventory/inventorypb/v1"
)

const (
	defaultListenAddr = ":9090"
	pingTimeout       = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("inventory exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err = pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	addr := cmp.Or(os.Getenv("LISTEN_ADDR"), defaultListenAddr)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	grpcServer := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(grpcServer, server.New(store.New(pool)))

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()
	return grpcServer.Serve(lis)
}
