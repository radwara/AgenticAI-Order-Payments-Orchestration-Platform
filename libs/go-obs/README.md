# go-obs

Observability helpers for Go services (T-019).

- `obs.Setup(ctx, obs.Config{ServiceName: ...})` installs W3C trace-context propagation and, when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, OTLP trace and metric exporters. With no endpoint it is a no-op and never fails. Call the returned shutdown on exit.
- `obs.GRPCServerOption()` / `obs.GRPCClientOption()` and `obs.HTTPMiddleware` / `obs.HTTPTransport` propagate trace context (call after `Setup`).
- `obs.NewLogger(w, level)`: JSON `slog` logger adding `trace_id`/`span_id`; log with `InfoContext(ctx, ...)`.
- `obs.NewHealth()`: `Register(name, check)`, `Mount(mux)` serves `/healthz` (always 200) and `/readyz` (503 if any check fails).

Run `make test` and `make lint` in this directory.
