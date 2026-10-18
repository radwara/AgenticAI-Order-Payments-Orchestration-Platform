---
id: T-019
title: "[T-019] OpenTelemetry and health helpers for Go"
labels: phase:1, status:needs-approval, area:platform, lang:go, size:S
---
## Why
PRD 12.1: every request traceable end to end from day one. Java counterpart is T-025.

## What
Provide `libs/go-obs` that: configures OTel tracing and metrics export via OTLP from env (`OTEL_EXPORTER_OTLP_ENDPOINT`), propagates W3C trace context on gRPC and HTTP (interceptors/middleware), emits JSON logs (`log/slog`) including `trace_id`, and exposes `/healthz` (liveness) and `/readyz` (readiness with pluggable dependency checks).

## Constraints
Go 1.23, standard library plus only dependencies named here; `gofmt` and `golangci-lint` clean. Only `go.opentelemetry.io/otel` family and `otelgrpc`/`otelhttp` contrib packages. No vendor-specific agents.

## Out of scope
Java library (T-025), dashboards, collector deployment, sampling policy.

## Acceptance criteria (validations)
- [ ] With a test OTLP receiver, a request produces a span and trace context propagates across a two-service in-process gRPC test
- [ ] Log lines are valid JSON and contain trace_id inside a span
- [ ] /readyz returns 503 when a registered dependency check fails, 200 when it recovers
- [ ] Starting with no OTLP endpoint set does not crash the service
- [ ] Coverage >= 85% on package

## Files in scope
`libs/go-obs/**`

## Depends on
T-001
