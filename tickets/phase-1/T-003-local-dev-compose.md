---
id: T-003
title: "[T-003] Local dev stack with Docker Compose"
labels: phase:1, status:needs-approval, area:platform, size:S
---
## Why
Developers and agents need Postgres, Kafka and Redis locally for integration tests. SPEC-01.

## What
Add `deploy/local/docker-compose.yml` with Postgres 16 (one DB per service created by an init script: `orders`, `payments`, `inventory`, `pricing`), single-broker Kafka (KRaft), Redis 7. Add `make dev-up` and `make dev-down`. Healthchecks on all services.

## Constraints
Images pinned by version tag. Credentials are obviously fake local-only values read from `deploy/local/.env.example`. Ports bound to 127.0.0.1.

## Out of scope
Application services, Kubernetes manifests.

## Acceptance criteria (validations)
- [ ] `make dev-up` brings all containers to healthy (script `deploy/local/verify.sh` checks psql, kafka topic list, redis ping)
- [ ] The four databases exist after init
- [ ] `make dev-down` removes containers and leaves no ports bound

## Files in scope
`deploy/local/**`, `Makefile`

## Depends on
T-001
