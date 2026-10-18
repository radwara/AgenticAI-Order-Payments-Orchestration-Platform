# SPEC-00 Architecture and conventions
**PRD refs:** §1, §6, §8, §10 · **Status:** draft for approval

## Decisions
- Domain-oriented microservices, event-driven, API-first. gRPC internally, REST/JSON externally, Kafka for domain events.
- Go for edge/data-plane (gateway, identity, tenant, pricing, inventory, notification). Java 21 + Spring Boot for rule-heavy domain (order, payment, fulfilment, reporting).
- PostgreSQL per service; no cross-service DB access. Redis for cache/idempotency.
- Monorepo (PRD open question resolved here for v1): `services/<name>`, `libs/<lang>-<name>`, `contracts/` (OpenAPI + proto), `deploy/`.
- Money: integer minor units + ISO-4217 code. Never floats.
- Every service: health/readiness endpoints, OpenTelemetry traces/metrics/logs, structured JSON logs, config from env.

## Decision: database (ADR-001, accepted)
- **PostgreSQL** for all services' transactional data (PRD §8). Services need row locks, constraints, `SKIP LOCKED`, triggers and jsonb.
- **Production:** managed Postgres from the chosen cloud (RDS/Aurora, Cloud SQL/AlloyDB or Azure Database for PostgreSQL), private networking, credentials via External Secrets, connection pooler (PgBouncer or cloud proxy) in front. Databases are not run inside the cluster in v1.
- **Dev/staging:** Docker Compose locally (T-003); CloudNativePG operator is an optional in-cluster alternative.
- **Layout:** one instance to start with a separate database and user per service; no shared tables or credentials. Payments moves to its own instance first.
- **Migrations:** Flyway (Java) and golang-migrate (Go), run by the CI/CD pipeline, not at pod startup.
- **Cloud provider:** still open; no code depends on it.

## Phase 1 scope (this repo's first milestone)
Walking skeleton: Pricing, Inventory, Order (with saga), Payment (mock provider + ledger) running locally and deployable via a base Helm chart. Gateway/identity/tenant, webhooks, real providers, multi-region come in later phases (see `docs/roadmap.md`).

## Acceptance for the spec
Each downstream spec cites requirement IDs from the PRD; every ticket cites a spec section.
