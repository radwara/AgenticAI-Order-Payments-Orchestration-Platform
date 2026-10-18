# SPEC-01 Platform, CI and Kubernetes baseline
**PRD refs:** §9, §13 · **Tickets:** T-001, T-002, T-003, T-018, T-019, T-022, T-023, T-024, T-025

- Repo layout and `make` targets for build/test/lint per service.
- Path-filtered CI per language; required checks listed in `agents/guardrails.md`.
- Local dev: docker-compose with Postgres, Kafka (single broker), Redis.
- Helm base chart ("library") encoding §9.2 workload standards: non-root, read-only rootfs, dropped caps, seccomp RuntimeDefault, probes, PDB, resource requests/limits, default-deny NetworkPolicy.
- Observability baseline: OTel SDK wiring helper for Go and Java, `/healthz` `/readyz`.
