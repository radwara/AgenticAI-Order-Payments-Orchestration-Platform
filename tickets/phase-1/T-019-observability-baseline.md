---
id: T-019
title: "[T-019] OpenTelemetry and health endpoint helpers"
labels: phase:1, status:needs-approval, area:platform, lang:go, lang:java, size:M
---
## Why
PRD §12.1: every request traceable end to end from day one.

## What
Provide `libs/go-obs` (Go) and `libs/java-obs` (Spring Boot starter) that: configure OTel tracing and metrics export via OTLP from env (`OTEL_EXPORTER_OTLP_ENDPOINT`), propagate W3C trace context on gRPC and HTTP, emit JSON logs including `trace_id`, and expose `/healthz` (liveness) and `/readyz` (readiness with pluggable dependency checks).

## Constraints
Go: `go.opentelemetry.io/otel` family only. Java: `io.opentelemetry` and Micrometer tracing bridge only. No vendor-specific agents.

## Out of scope
Dashboards, collector deployment, sampling policy.

## Acceptance criteria (validations)
- [ ] With a test OTLP receiver, a request produces a span and trace context propagates across a two-service in-process test (Go and Java)
- [ ] Log lines are valid JSON and contain trace_id inside a span
- [ ] /readyz returns 503 when a registered dependency check fails, 200 when it recovers
- [ ] Starting with no OTLP endpoint set does not crash the service

## Files in scope
`libs/go-obs/**`, `libs/java-obs/**`

## Depends on
T-001
