# SPEC-00 Architecture and conventions
**PRD refs:** §1, §6, §8, §10 · **Status:** draft for approval

## Decisions
- Domain-oriented microservices, event-driven, API-first. gRPC internally, REST/JSON externally, Kafka for domain events.
- Go for edge/data-plane (gateway, identity, tenant, pricing, inventory, notification). Java 21 + Spring Boot for rule-heavy domain (order, payment, fulfilment, reporting).
- PostgreSQL per service; no cross-service DB access. Redis for cache/idempotency.
- Monorepo (PRD open question resolved here for v1): `services/<name>`, `libs/<lang>-<name>`, `contracts/` (OpenAPI + proto), `deploy/`.
- Money: integer minor units + ISO-4217 code. Never floats.
- Every service: health/readiness endpoints, OpenTelemetry traces/metrics/logs, structured JSON logs, config from env.

## Phase 1 scope (this repo's first milestone)
Walking skeleton: Pricing, Inventory, Order (with saga), Payment (mock provider + ledger) running locally and deployable via a base Helm chart. Gateway/identity/tenant, webhooks, real providers, multi-region come in later phases (see `docs/roadmap.md`).

## Acceptance for the spec
Each downstream spec cites requirement IDs from the PRD; every ticket cites a spec section.
