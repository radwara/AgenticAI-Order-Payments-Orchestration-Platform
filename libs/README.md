# libs/

Shared libraries, one directory per library named `libs/<lang>-<name>` (for example `libs/go-money`, `libs/java-outbox`).

Libraries are built, tested and linted by the root `Makefile` (`make build`, `make test`, `make lint`) the same way as `services/*`: if a library has its own `Makefile`, the matching target is run; otherwise it is skipped.

Other top-level directories: `services/` (deployables), `contracts/` (OpenAPI and proto), `deploy/` (Helm and deployment config).
