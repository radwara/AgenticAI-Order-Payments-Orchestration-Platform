# SPEC-04 Inventory Service (Go)
**PRD refs:** §5.3 INV-1..3 · **Tickets:** T-007, T-008, T-009

- Stock per SKU and location. Reserve, release, commit are atomic and idempotent per reservation id.
- Never oversell under concurrency: row-level locks (`SELECT ... FOR UPDATE`) or optimistic version column with retry.
- Reservations expire after configurable TTL (default 15 min); expiry releases stock.
