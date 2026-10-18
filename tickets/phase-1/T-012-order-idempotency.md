---
id: T-012
title: "[T-012] Idempotent order creation"
labels: phase:1, status:needs-approval, lang:java, area:order, size:S
---
## Why
ORD-4: retries must not create duplicate orders or duplicate charges. SPEC-02.

## What
Support header `Idempotency-Key` on create. Store `(merchant_id, key, request_hash, response_status, response_body, created_at)` in Postgres in the same transaction as the order. Same key and same body -> replay original response with header `Idempotent-Replayed: true`. Same key, different body -> 422. Retention 24 h with a scheduled cleanup. Missing header -> 400.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Postgres is the source of truth (Redis not required in this ticket). No new dependencies.

## Out of scope
Redis caching layer, other endpoints.

## Acceptance criteria (validations)
- [ ] Two sequential creates with same key and body yield one order row and identical responses
- [ ] Two concurrent creates with same key yield exactly one order (integration test with 20 threads)
- [ ] Same key, different body -> 422 problem+json
- [ ] Rows older than 24 h are removed by cleanup (fake clock test)
- [ ] Idempotency scope is per merchant: same key from two merchants creates two orders

## Files in scope
`services/order/**`

## Depends on
T-011
