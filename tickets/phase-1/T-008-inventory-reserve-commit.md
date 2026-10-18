---
id: T-008
title: "[T-008] Atomic reserve, release and commit"
labels: phase:1, status:needs-approval, lang:go, area:inventory, size:M
---
## Why
INV-1, INV-3: never oversell, even under concurrency. SPEC-04.

## What
Add RPCs `Reserve(reservation_id, items[])`, `Release(reservation_id)`, `Commit(reservation_id)`. Reserve is all-or-nothing across items and idempotent by `reservation_id` (same id + same items returns same result; same id + different items -> ALREADY_EXISTS). Use a `reservations` table and row-level locks (`SELECT ... FOR UPDATE`, items locked in sorted key order to avoid deadlocks).

## Constraints
Go 1.23, standard library plus only dependencies named here; `gofmt` and `golangci-lint` clean. Single DB transaction per RPC. No distributed locks. No new dependencies.

## Out of scope
TTL expiry (T-009), events.

## Acceptance criteria (validations)
- [ ] Insufficient stock on any item -> FAILED_PRECONDITION and nothing reserved
- [ ] 100 concurrent Reserve calls for 10 units of stock succeed exactly 10 times (integration test with real Postgres, `-race`)
- [ ] Release returns reserved units; Commit decrements on_hand and reserved
- [ ] Repeating Reserve/Release/Commit with same id is a no-op returning the original outcome
- [ ] Two requests locking items in opposite order never deadlock (test)

## Files in scope
`services/inventory/**`, `contracts/inventory/**`

## Depends on
T-007
