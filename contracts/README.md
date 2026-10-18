# contracts/

API contracts shared across services: OpenAPI specs for external REST/JSON and proto files for internal gRPC (SPEC-00). Contract changes are subject to the breaking-change check described in `agents/guardrails.md`.

Not a build module: the root `Makefile` iterates only over `services/*` and `libs/*`.

Other top-level directories: `services/` (deployables), `libs/` (shared libraries), `deploy/` (Helm and deployment config).
