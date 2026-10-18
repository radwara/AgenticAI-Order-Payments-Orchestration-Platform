---
id: T-025
title: "[T-025] OpenTelemetry and health starter for Java"
labels: phase:1, status:needs-approval, area:platform, lang:java, size:S
---
## Why
PRD 12.1: end-to-end traceability. Go counterpart is T-019.

## What
Provide `libs/java-obs`, a Spring Boot auto-configuration starter that: configures OTel tracing and metrics export via OTLP from env, propagates W3C trace context on HTTP and gRPC clients/servers, emits JSON logs including `trace_id` (via MDC), and exposes liveness and readiness through Spring Actuator health groups with pluggable dependency checks.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Only `io.opentelemetry` and Micrometer tracing bridge. No vendor agents.

## Out of scope
Go library (T-019), dashboards, collector deployment.

## Acceptance criteria (validations)
- [ ] With a test OTLP receiver, a request produces a span and trace context propagates across a two-service in-process test
- [ ] Log lines are valid JSON and contain trace_id inside a span
- [ ] Readiness reports DOWN (503) when a registered dependency check fails and UP when it recovers
- [ ] Starting with no OTLP endpoint set does not crash the service

## Files in scope
`libs/java-obs/**`

## Depends on
T-001
