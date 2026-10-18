---
id: T-024
title: "[T-024] Postgres deployment manifests and wiring"
labels: phase:1, status:needs-approval, area:platform, size:M
---
## Why
ADR-001 in SPEC-00: managed Postgres in production, in-cluster alternative for dev/staging, pooled, credentials via External Secrets.

## What
Add `deploy/postgres/`: (1) CloudNativePG `Cluster` and `Pooler` (PgBouncer) manifests for dev/staging with one database and role per service; (2) `ExternalSecret` templates mapping a cloud secret to each service's `DB_URL`/`DB_USER`/`DB_PASSWORD`; (3) extend `deploy/charts/service-base` with optional `database` values that mount those secrets as env vars; (4) `docs/postgres.md` describing managed-cloud production setup (private networking, pooler, backups, migration job).

## Constraints
Manifests only; no cluster required. Validate with `kubeconform` using the CNPG and External Secrets CRD schemas (pinned versions). No credentials in files; example secret names only.

## Out of scope
Terraform for managed cloud databases, backups tuning, HA sizing.

## Acceptance criteria (validations)
- [ ] kubeconform strict validation passes for all manifests
- [ ] Each service has its own role and database; no role has access to another service's database (asserted by a script on rendered SQL/CR)
- [ ] Chart renders DB env vars from secretKeyRef only (no plaintext values)
- [ ] docs/postgres.md covers managed production, pooler and migration-job guidance

## Files in scope
`deploy/postgres/**`, `deploy/charts/service-base/**`, `docs/postgres.md`, `scripts/validate-postgres.sh`

## Depends on
T-018
