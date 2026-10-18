---
id: T-016
title: "[T-016] Payment service: authorize, capture, void, refund"
labels: phase:1, status:needs-approval, lang:java, area:payment, size:M
---
## Why
PAY-1: service layer over the provider adapter with persisted payment state. SPEC-03.

## What
Add REST API `POST /v1/payments` (authorize), `/{id}/capture`, `/{id}/void`, `/{id}/refund` with Postgres persistence (`payments`, `payment_events`) and an idempotency key per operation. Payment states: AUTHORIZED, CAPTURED, VOIDED, PARTIALLY_REFUNDED, REFUNDED, FAILED with an explicit transition table. Uses `MockProvider` via `PaymentProvider` bean.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Allowed: spring-boot-starter-web, -validation, -data-jpa, flyway, postgresql, testcontainers. Money as minor units.

## Out of scope
Ledger posting (T-017), retries/circuit breaker, webhooks.

## Acceptance criteria (validations)
- [ ] Authorize with tok_ok -> AUTHORIZED; tok_declined -> 402 problem+json with reason
- [ ] Void after capture -> 409; partial refunds accumulate and cannot exceed captured
- [ ] Repeating any operation with the same idempotency key returns the original result without a second provider call (verified with a spy)
- [ ] Illegal transition table covered exhaustively
- [ ] Integration test on real Postgres

## Files in scope
`services/payment/**`

## Depends on
T-015, T-003
