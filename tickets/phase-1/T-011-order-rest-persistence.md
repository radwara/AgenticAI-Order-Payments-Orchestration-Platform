---
id: T-011
title: "[T-011] Order create, read and cancel (REST + Postgres)"
labels: phase:1, status:needs-approval, lang:java, area:order, size:M
---
## Why
ORD-1: core order API per contract. SPEC-02, contract T-004.

## What
Implement `POST /v1/orders`, `GET /v1/orders/{id}`, `POST /v1/orders/{id}/cancel` per `contracts/order/v1/openapi.yaml` with Spring Web, Spring Data JPA, Flyway migrations (`orders`, `order_lines`), Bean Validation. New orders are created in CREATED. Cancel uses the state machine. Errors as RFC 9457 problem+json.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Allowed: spring-boot-starter-web, -validation, -data-jpa, -actuator, flyway, postgresql, testcontainers, JUnit. Money stored as `amount_minor BIGINT` and `currency CHAR(3)`; no floating point.

## Out of scope
List endpoint (T-013), idempotency (T-012), events (T-014), saga.

## Acceptance criteria (validations)
- [ ] Create returns 201 with Location; Get returns the same body; unknown id -> 404 problem+json
- [ ] Invalid payload (negative qty, bad currency) -> 400 with field errors
- [ ] Cancel from CREATED -> CANCELLED; cancel from COMPLETED -> 409
- [ ] Responses validate against the OpenAPI contract (contract test)
- [ ] Testcontainers integration test covers the flow on real Postgres

## Files in scope
`services/order/**`

## Depends on
T-004, T-010, T-003
