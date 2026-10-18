---
id: T-004
title: "[T-004] Order API v1 OpenAPI contract"
labels: phase:1, status:needs-approval, area:order, size:S
---
## Why
Contract-first lets code and review agents verify against one source. PRD §7.1, §7.2, ORD-1, ORD-4, ORD-6.

## What
Write `contracts/order/v1/openapi.yaml` (OpenAPI 3.1) for: `POST /v1/orders`, `GET /v1/orders/{id}`, `GET /v1/orders` (cursor pagination: `limit`, `cursor`, response `next_cursor`), `POST /v1/orders/{id}/cancel`. Include `Idempotency-Key` header on create, money as `{amount_minor:int64, currency:ISO-4217}`, RFC 9457 problem+json errors, order states from SPEC-02.

## Constraints
No code generation yet. Validates with `redocly lint` (CI-run via npx pinned version in a script).

## Out of scope
Implementation, other services' contracts.

## Acceptance criteria (validations)
- [ ] `scripts/lint-openapi.sh` passes
- [ ] All four operations present with 2xx and error responses
- [ ] State enum equals SPEC-02 states exactly (checked by script)

## Files in scope
`contracts/order/v1/openapi.yaml`, `scripts/lint-openapi.sh`

## Depends on
T-001
