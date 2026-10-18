---
id: T-013
title: "[T-013] Cursor-paginated order listing"
labels: phase:1, status:needs-approval, lang:java, area:order, size:S
---
## Why
ORD-6: stable, scalable list endpoint. SPEC-02.

## What
Implement `GET /v1/orders?limit&cursor&status&created_after&sort` with keyset pagination on `(created_at, id)`. Cursor is opaque base64url of the last key and the sort/filter hash. `limit` default 50, max 200. Response includes `next_cursor` (null on last page).

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. No OFFSET pagination. Index on `(merchant_id, created_at, id)` added via Flyway. No new dependencies.

## Out of scope
Full-text search, other filters.

## Acceptance criteria (validations)
- [ ] Paging through 1,000 seeded orders with limit 37 returns each order exactly once in order
- [ ] Inserting orders between page requests never duplicates or skips already-seen orders
- [ ] Tampered or mismatched cursor -> 400
- [ ] limit above 200 -> clamped or 400 as specified in the contract (test pins behaviour)
- [ ] EXPLAIN in integration test shows index scan, not sort of the full table

## Files in scope
`services/order/**`, `contracts/order/v1/openapi.yaml`

## Depends on
T-011
