# deploy/

Deployment assets: the base Helm chart and per-service values, and later environment configuration (SPEC-01). Local docker-compose and CI are introduced by their own tickets.

Not a build module: the root `Makefile` iterates only over `services/*` and `libs/*`.

Other top-level directories: `services/` (deployables), `libs/` (shared libraries), `contracts/` (OpenAPI and proto).
