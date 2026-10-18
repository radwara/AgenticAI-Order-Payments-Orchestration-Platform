package obs

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// NewLogger returns a JSON slog logger that adds top-level trace_id and span_id
// to every record logged with a context carrying a valid span (use the
// *Context methods such as InfoContext).
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(&traceHandler{base: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

// op is a deferred WithAttrs/WithGroup call. Ops are replayed after the trace
// ids are added so the ids stay top-level even when the logger uses groups.
type op struct {
	group string
	attrs []slog.Attr
}

type traceHandler struct {
	base slog.Handler
	ops  []op
}

func (t *traceHandler) build(extra []slog.Attr) slog.Handler {
	h := t.base
	if len(extra) > 0 {
		h = h.WithAttrs(extra)
	}
	for _, o := range t.ops {
		if o.group != "" {
			h = h.WithGroup(o.group)
		} else {
			h = h.WithAttrs(o.attrs)
		}
	}
	return h
}

func (t *traceHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return t.base.Enabled(ctx, l)
}

func (t *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	var extra []slog.Attr
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		extra = []slog.Attr{slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String())}
	}
	return t.build(extra).Handle(ctx, r)
}

func (t *traceHandler) with(o op) slog.Handler {
	ops := append(append([]op(nil), t.ops...), o)
	return &traceHandler{base: t.base, ops: ops}
}

func (t *traceHandler) WithAttrs(a []slog.Attr) slog.Handler { return t.with(op{attrs: a}) }

func (t *traceHandler) WithGroup(n string) slog.Handler {
	if n == "" {
		return t
	}
	return t.with(op{group: n})
}
