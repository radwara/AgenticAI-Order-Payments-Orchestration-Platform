# go-obs

Observability helpers for Go services (T-019).

- `obs.Setup(ctx, obs.Config{ServiceName: ...})` installs W3C trace-context propagation and, when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, OTLP trace and metric exporters. With no endpoint it is a no-op and never fails. Only `OTEL_EXPORTER_OTLP_ENDPOINT` triggers export; signal-specific endpoint vars (`..._TRACES_ENDPOINT`, `..._METRICS_ENDPOINT`) and `OTEL_SDK_DISABLED` are not consulted. Call the returned shutdown on exit; it is bounded by `Config.ShutdownTimeout` (default 5s) even if the passed context has no deadline.
- `obs.GRPCServerOption()` / `obs.GRPCClientOption()` and `obs.HTTPMiddleware` / `obs.HTTPTransport` propagate trace context (call after `Setup`).
- `obs.NewLogger(w, level)`: JSON `slog` logger adding `trace_id`/`span_id`; log with `InfoContext(ctx, ...)`.
- `obs.NewHealth()`: `Register(name, check)`, `Mount(mux)` serves `/healthz` (always 200) and `/readyz` (503 if any check fails or exceeds the 2s probe timeout, even if the check ignores its context; the body lists check names with `ok`/`unavailable` only, details go to the logger set via `SetLogger`, default `slog.Default()`). A check still hung from a previous probe is not started again.

Run `make test` and `make lint` in this directory.
