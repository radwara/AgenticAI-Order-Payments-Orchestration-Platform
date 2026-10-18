# services/order

Order domain model (SPEC-02, ORD-2). Pure Java 21, no Spring and no persistence yet; REST, DB and events come in later tickets.

## State machine

`OrderStateMachine` holds the explicit transition table. Any pair not listed throws `IllegalTransitionException`; it is never a silent no-op. `Order` changes status only through the state machine, and once COMPLETED every mutator throws `OrderCompletedException`.

| From | Allowed targets |
|---|---|
| CREATED | PRICED, CANCELLED, FAILED |
| PRICED | RESERVED, CANCELLED, FAILED |
| RESERVED | PAID, CANCELLED, FAILED |
| PAID | FULFILLING, CANCELLED, FAILED |
| FULFILLING | COMPLETED, FAILED |
| COMPLETED, CANCELLED, FAILED | none (terminal) |

Cancel is allowed only before fulfilment (CREATED, PRICED, RESERVED, PAID). FAILED is reachable from every non-terminal state so saga failures can always be recorded.

## Build

`make build`, `make test` (runs `mvn -B verify`, JaCoCo gate: 100% line and branch on `OrderStateMachine`; report at `target/site/jacoco/jacoco.xml`) and `make lint` (Spotless, google-java-format). Needs JDK 21 and Maven.
