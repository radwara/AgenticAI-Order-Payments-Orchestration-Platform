---
id: T-009
title: "[T-009] Reservation expiry"
labels: phase:1, status:needs-approval, lang:go, area:inventory, size:S
---
## Why
INV-2: abandoned checkouts must free stock automatically. SPEC-04.

## What
Reservations get `expires_at = now + TTL` (config `RESERVATION_TTL`, default 15m). Add a background reaper (interval config, default 30s) that releases expired, uncommitted reservations in batches using `FOR UPDATE SKIP LOCKED`. Commit after expiry fails with FAILED_PRECONDITION.

## Constraints
Go 1.23, standard library plus only dependencies named here; `gofmt` and `golangci-lint` clean. Time source injected (interface) so tests do not sleep. Reaper must be safe to run on multiple replicas.

## Out of scope
Metrics dashboards, events on expiry.

## Acceptance criteria (validations)
- [ ] With a fake clock, a reservation older than TTL is released by one reaper tick and stock is available again
- [ ] Two reaper instances running concurrently release each reservation exactly once
- [ ] Committed reservations are never reaped
- [ ] Commit on expired reservation fails

## Files in scope
`services/inventory/**`

## Depends on
T-008
