package obs

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	colmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// fakeCollector is an in-process OTLP trace receiver.
type fakeCollector struct {
	coltrace.UnimplementedTraceServiceServer
	mu    sync.Mutex
	spans []spanRec
}

type spanRec struct{ name, traceID string }

func (f *fakeCollector) Export(_ context.Context, r *coltrace.ExportTraceServiceRequest) (*coltrace.ExportTraceServiceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, rs := range r.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				f.spans = append(f.spans, spanRec{s.Name, string(s.TraceId)})
			}
		}
	}
	return &coltrace.ExportTraceServiceResponse{}, nil
}

// fakeMetrics is an in-process OTLP metrics receiver that counts exports.
type fakeMetrics struct {
	colmetrics.UnimplementedMetricsServiceServer
	n atomic.Int32
}

func (f *fakeMetrics) Export(context.Context, *colmetrics.ExportMetricsServiceRequest) (*colmetrics.ExportMetricsServiceResponse, error) {
	f.n.Add(1)
	return &colmetrics.ExportMetricsServiceResponse{}, nil
}

func (f *fakeCollector) snapshot() []spanRec {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]spanRec(nil), f.spans...)
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// healthFunc adapts a func to the gRPC health service so tests need no generated code.
type healthFunc func(context.Context) (*grpc_health_v1.HealthCheckResponse, error)

func (f healthFunc) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return f(ctx)
}
func (healthFunc) Watch(*grpc_health_v1.HealthCheckRequest, grpc.ServerStreamingServer[grpc_health_v1.HealthCheckResponse]) error {
	return nil
}

func serve(t *testing.T, h grpc_health_v1.HealthServer) string {
	t.Helper()
	l := listen(t)
	s := grpc.NewServer(GRPCServerOption())
	grpc_health_v1.RegisterHealthServer(s, h)
	go func() { _ = s.Serve(l) }()
	t.Cleanup(s.Stop)
	return l.Addr().String()
}

func dial(t *testing.T, addr string) grpc_health_v1.HealthClient {
	t.Helper()
	cc, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), GRPCClientOption())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return grpc_health_v1.NewHealthClient(cc)
}

func TestSetupExportsSpansAndPropagatesAcrossTwoGRPCServices(t *testing.T) {
	col := &fakeCollector{}
	cl := listen(t)
	cs := grpc.NewServer()
	met := &fakeMetrics{}
	coltrace.RegisterTraceServiceServer(cs, col)
	colmetrics.RegisterMetricsServiceServer(cs, met)
	go func() { _ = cs.Serve(cl) }()
	t.Cleanup(cs.Stop)

	t.Setenv(EndpointEnv, "http://"+cl.Addr().String())
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	shutdown, err := Setup(context.Background(), Config{ServiceName: "test", ServiceVersion: "0", MetricInterval: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	// Service B records the trace id it sees; service A calls B.
	var bTrace string
	addrB := serve(t, healthFunc(func(ctx context.Context) (*grpc_health_v1.HealthCheckResponse, error) {
		_, sp := otel.Tracer("b").Start(ctx, "b-work")
		bTrace = sp.SpanContext().TraceID().String()
		sp.End()
		return &grpc_health_v1.HealthCheckResponse{}, nil
	}))
	clientB := dial(t, addrB)
	addrA := serve(t, healthFunc(func(ctx context.Context) (*grpc_health_v1.HealthCheckResponse, error) {
		return clientB.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	}))

	ctx, root := otel.Tracer("root").Start(context.Background(), "request")
	if _, err := dial(t, addrA).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	root.End()
	want := root.SpanContext().TraceID().String()
	if bTrace != want {
		t.Fatalf("trace not propagated to service B: got %q want %q", bTrace, want)
	}

	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := shutdown(sctx); err != nil {
		t.Fatal(err)
	}
	if met.n.Load() == 0 {
		t.Fatal("no OTLP metrics export received")
	}
	spans := col.snapshot()
	names := map[string]bool{}
	for _, s := range spans {
		if hex := toHex(s.traceID); hex != want {
			t.Errorf("span %q has trace %s, want %s", s.name, hex, want)
		}
		names[s.name] = true
	}
	if !names["request"] || !names["b-work"] || len(spans) < 5 {
		t.Fatalf("expected request, b-work and gRPC spans, got %v", spans)
	}
}

func toHex(b string) string {
	const h = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for i := 0; i < len(b); i++ {
		out = append(out, h[b[i]>>4], h[b[i]&15])
	}
	return string(out)
}

func TestSetupWithoutEndpointDoesNotFail(t *testing.T) {
	t.Setenv(EndpointEnv, "")
	shutdown, err := Setup(context.Background(), Config{ServiceName: "svc"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	_, sp := otel.Tracer("x").Start(context.Background(), "noop")
	sp.End()
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(otel.GetTextMapPropagator().Fields()) == 0 {
		t.Fatal("propagator not installed")
	}
}

func TestSetupUnreachableEndpointDoesNotCrash(t *testing.T) {
	t.Setenv(EndpointEnv, "http://127.0.0.1:1")
	shutdown, err := Setup(context.Background(), Config{ServiceName: "svc"})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = shutdown(ctx) // export failure is acceptable; must not panic
}

func TestHTTPPropagation(t *testing.T) {
	t.Setenv(EndpointEnv, "")
	if _, err := Setup(context.Background(), Config{ServiceName: "svc"}); err != nil {
		t.Fatal(err)
	}
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	var got string
	srv := httptest.NewServer(HTTPMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = trace.SpanContextFromContext(r.Context()).TraceID().String()
	}), "op"))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")
	resp, err := (&http.Client{Transport: HTTPTransport(nil)}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got != traceID {
		t.Fatalf("trace id = %q, want %q", got, traceID)
	}
}
