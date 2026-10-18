# SPEC-02 Order Service (Java 21)
**PRD refs:** §5.1 ORD-1..6, §7.1, §7.3 · **Tickets:** T-004, T-010..T-014, T-020, T-021

- States: CREATED, PRICED, RESERVED, PAID, FULFILLING, COMPLETED, CANCELLED, FAILED. Transitions only via the state machine; illegal transition is an error.
- REST: create/read/list/cancel; cursor pagination; `Idempotency-Key` header, scope merchant+key, 24 h retention, same key + same body returns original response, same key + different body returns 422.
- Events (`OrderCreated`, `OrderPaid`, `OrderCancelled`) written to an outbox table in the same transaction as the state change; a relay publishes to Kafka at-least-once; consumers dedupe by event id.
- Saga: reserve inventory, authorize payment, confirm. Failure after any step triggers compensations in reverse order; compensation is idempotent.
