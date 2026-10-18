// Package obs wires OpenTelemetry tracing and metrics, JSON logging with trace
// ids, W3C context propagation and health endpoints for Go services.
package obs

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// EndpointEnv is the standard env var naming the OTLP collector.
const EndpointEnv = "OTEL_EXPORTER_OTLP_ENDPOINT"

// Config configures Setup.
type Config struct {
	ServiceName    string
	ServiceVersion string
	// MetricInterval overrides the periodic metric export interval (default 10s).
	MetricInterval time.Duration
}

// Setup installs the W3C trace-context/baggage propagator and, when
// OTEL_EXPORTER_OTLP_ENDPOINT is set, OTLP trace and metric exporters as the
// global providers. With no endpoint it only installs the propagator (spans are
// no-ops) and never fails. The returned shutdown flushes and stops exporters.
func Setup(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))

	if os.Getenv(EndpointEnv) == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	))
	if err != nil {
		return nil, err
	}

	// gRPC exporters connect lazily, so an unreachable collector does not crash startup.
	te, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(te), sdktrace.WithResource(res))

	me, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, err
	}
	interval := cfg.MetricInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(me, sdkmetric.WithInterval(interval))),
		sdkmetric.WithResource(res))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}
