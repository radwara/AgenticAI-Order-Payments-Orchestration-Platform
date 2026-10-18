---
id: T-010
title: "[T-010] Order domain model and state machine"
labels: phase:1, status:needs-approval, lang:java, area:order, size:S
---
## Why
ORD-2: only legal transitions; foundation for saga and API. SPEC-02.

## What
Create Maven module `services/order` with package `domain`: `Order` aggregate, `OrderStatus` enum (CREATED, PRICED, RESERVED, PAID, FULFILLING, COMPLETED, CANCELLED, FAILED) and `OrderStateMachine` with an explicit transition table (document it in the class Javadoc and in `services/order/README.md`). No Spring, no persistence in this ticket.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Pure Java domain code; JUnit 5 and AssertJ only.

## Out of scope
REST, DB, events.

## Acceptance criteria (validations)
- [ ] Every legal transition in the table succeeds; every other (from,to) pair throws `IllegalTransitionException` (exhaustive test over all state pairs)
- [ ] Terminal states COMPLETED, CANCELLED, FAILED allow no outgoing transitions
- [ ] Cancel allowed from CREATED, PRICED, RESERVED, PAID (before fulfilment); not from FULFILLING or COMPLETED
- [ ] Order is immutable once COMPLETED (mutators throw)
- [ ] Coverage 100% on state machine

## Files in scope
`services/order/pom.xml`, `services/order/src/main/java/**/domain/**`, `services/order/src/test/java/**/domain/**`, `services/order/README.md`

## Depends on
T-001
