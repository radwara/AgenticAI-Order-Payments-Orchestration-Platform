---
id: T-021
title: "[T-021] Order saga: compensating actions"
labels: phase:1, status:needs-approval, lang:java, area:order, size:M
---
## Why
ORD-3: failures must leave no orphan reservations or charges. SPEC-02.

## What
On failure at any saga step run compensations in reverse order: payment declined -> release inventory reservation -> order FAILED (event `OrderFailed`); customer cancel after RESERVED -> release reservation; cancel after PAID -> void or refund then release. Compensations are idempotent and retried until success or escalation (`saga_state = COMPENSATION_STUCK` after N attempts, metric emitted).

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. No new dependencies.

## Out of scope
Manual operator tooling, dead-letter UI.

## Acceptance criteria (validations)
- [ ] Pricing failure -> order FAILED, no reservation made
- [ ] Payment declined -> reservation released exactly once, order FAILED
- [ ] Compensation call failing twice then succeeding ends in FAILED with reservation released (retry test)
- [ ] Crash during compensation and restart completes it without double release
- [ ] Cancel after PAID triggers refund and release; ledger nets to zero (e2e)

## Files in scope
`services/order/**`

## Depends on
T-020, T-017
