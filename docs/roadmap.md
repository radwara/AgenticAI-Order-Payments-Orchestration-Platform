# Roadmap
The PRD defines four phases of roughly one quarter each with named gates. Phase contents below are this repo's interpretation; adjust at approval.

| Phase | Theme | Gate |
|---|---|---|
| 1 | Foundation and walking skeleton: platform baseline, Pricing, Inventory, Order + saga, Payment on mock provider + ledger | Order created, priced, reserved, paid on mock, and completed end to end locally; CI and Helm baseline green |
| 2 | Edge and identity: gateway, identity, tenant, rate limits; webhooks and notifications; first real provider; retries and circuit breakers (PAY-3, PAY-5) | Secure external API in staging |
| 3 | Hardening: fulfilment, reporting, routing (PAY-6), reconciliation (PAY-7), load/resilience tests, SLOs, progressive delivery | GA decision |
| 4 | Second region, active-active | Multi-region failover drill passed |

Open questions carried from the PRD: cloud provider, service mesh in v1, workflow engine vs in-service saga (Phase 1 uses in-service saga), managed vs in-cluster Kafka, data residency, merchant dashboard.
