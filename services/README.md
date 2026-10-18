# services/

One directory per deployable microservice: `services/<name>` (for example `services/pricing`, `services/order`). Go for edge/data-plane services, Java 21 + Spring Boot for rule-heavy domain services (SPEC-00).

## Repo layout

| Path | Contents |
|---|---|
| `services/<name>` | Deployable services, one database each |
| `libs/<lang>-<name>` | Shared libraries, e.g. `libs/go-money` |
| `contracts/` | OpenAPI and proto contract files |
| `deploy/` | Helm charts and deployment config |

## Build, test, lint

The root `Makefile` provides `make build`, `make test` and `make lint`. Each iterates over `services/*` and `libs/*` and runs the same target in a module's own `Makefile` if it has one; modules without a `Makefile` (or without that target) are skipped. All modules run even if one fails, and the overall command then exits non-zero.

A service `Makefile` should define `build`, `test` and `lint`. Check the behaviour with `sh scripts/test-make.sh`.
