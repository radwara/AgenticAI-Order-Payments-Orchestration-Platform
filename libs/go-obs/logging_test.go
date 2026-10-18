package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestLogLinesAreJSONWithTraceIDInsideSpan(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, slog.LevelInfo).With("svc", "x").WithGroup("g")
	tp := sdktrace.NewTracerProvider()
	ctx, span := tp.Tracer("t").Start(context.Background(), "op")
	log.InfoContext(ctx, "inside")
	span.End()
	log.InfoContext(context.Background(), "outside")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	var in, out map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &in); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &out); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if in["trace_id"] != span.SpanContext().TraceID().String() || in["span_id"] == nil {
		t.Fatalf("missing ids: %v", in)
	}
	if _, ok := out["trace_id"]; ok {
		t.Fatalf("trace_id outside span: %v", out)
	}
}
