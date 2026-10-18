package obs

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
)

// GRPCServerOption instruments a gRPC server: extracts W3C trace context and
// creates a server span per RPC. Call after Setup.
func GRPCServerOption() grpc.ServerOption {
	return grpc.StatsHandler(otelgrpc.NewServerHandler())
}

// GRPCClientOption instruments a gRPC client connection: injects W3C trace context.
func GRPCClientOption() grpc.DialOption {
	return grpc.WithStatsHandler(otelgrpc.NewClientHandler())
}

// HTTPMiddleware wraps h with server-side tracing and context extraction.
func HTTPMiddleware(h http.Handler, operation string) http.Handler {
	return otelhttp.NewHandler(h, operation)
}

// HTTPTransport wraps rt (http.DefaultTransport if nil) to inject trace context.
func HTTPTransport(rt http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(rt)
}
