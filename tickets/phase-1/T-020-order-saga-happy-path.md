---
id: T-020
title: "[T-020] Order saga: price, reserve, pay, confirm"
labels: phase:1, status:needs-approval, lang:java, area:order, size:M
---
## Why
ORD-3: the Phase 1 gate is an order flowing end to end. SPEC-02.

## What
Implement `OrderSaga` in `services/order` orchestrating after create: call Pricing `Quote` (gRPC) -> state PRICED; Inventory `Reserve` -> RESERVED; Payment authorize+capture -> PAID; emit `OrderPaid`. Clients behind interfaces (`PricingClient`, `InventoryClient`, `PaymentClient`) with gRPC/HTTP implementations. Saga progress persisted in `saga_state` so a restart resumes where it stopped. Each outbound call carries a deterministic idempotency key derived from order id and step.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Allowed: grpc-java, protobuf-java. Saga logic must depend only on the client interfaces (ArchUnit).

## Out of scope
Compensation (T-021), timeouts/circuit breakers, fulfilment.

## Acceptance criteria (validations)
- [ ] With fake clients, an order walks CREATED->PRICED->RESERVED->PAID and `OrderPaid` is in the outbox
- [ ] Killing the process after the reserve step and restarting resumes at payment without re-reserving (test with same idempotency key)
- [ ] Each client call receives the same idempotency key on retry
- [ ] End-to-end test with real pricing, inventory and payment containers via docker-compose passes (`make e2e`)

## Files in scope
`services/order/**`, `deploy/local/**`, `Makefile`

## Depends on
T-006, T-008, T-016, T-014, T-012, T-023
