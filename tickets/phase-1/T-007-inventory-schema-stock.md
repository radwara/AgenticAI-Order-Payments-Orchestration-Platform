---
id: T-007
title: "[T-007] Inventory schema and stock level API"
labels: phase:1, status:needs-approval, lang:go, area:inventory, size:M
---
## Why
INV-1: stock per SKU and location is the base for reservations. SPEC-04.

## What
Create `services/inventory` (Go) with SQL migrations (golang-migrate) for tables `stock(sku, location, on_hand, reserved, version, updated_at)` with constraint `reserved <= on_hand AND reserved >= 0`, and gRPC/REST-less internal API `SetStock`, `GetStock` per `contracts/inventory/v1/inventory.proto`. Data access with `pgx`.

## Constraints
Go 1.23, standard library plus only dependencies named here; `gofmt` and `golangci-lint` clean. Dependencies allowed: `pgx/v5`, `golang-migrate`, grpc/protobuf, `testcontainers-go` for tests.

## Out of scope
Reserve/release/commit, expiry, bulk import (INV-4, P1).

## Acceptance criteria (validations)
- [ ] Migration up and down both succeed on a clean Postgres (integration test)
- [ ] DB rejects `reserved > on_hand` and negative values (constraint tests)
- [ ] SetStock then GetStock round-trips; unknown SKU -> NOT_FOUND
- [ ] Service builds into a distroless image via `Dockerfile` running as non-root

## Files in scope
`services/inventory/**`, `contracts/inventory/**`

## Depends on
T-003
