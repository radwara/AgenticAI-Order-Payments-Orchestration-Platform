---
id: T-001
title: "[T-001] Monorepo layout and Makefile"
labels: phase:1, status:needs-approval, area:platform, size:S
---
## Why
Every later ticket needs agreed paths and one command to build/test. SPEC-00, SPEC-01.

## What
Create directories `services/`, `libs/`, `contracts/`, `deploy/` with README stubs and a root `Makefile` exposing `make build`, `make test`, `make lint` that iterate over `services/*` and `libs/*` and invoke each module's own Makefile if present (no-op otherwise). Add `.editorconfig` and `.gitignore` for Go, Java, IDE files.

## Constraints
No language code. Make only; no new tools. Must work on macOS and Linux with POSIX shell.

## Out of scope
CI, docker-compose, any service code.

## Acceptance criteria (validations)
- [ ] `make build test lint` exits 0 on an empty tree
- [ ] Adding a dummy `services/x/Makefile` with a failing `test` target makes `make test` fail (shown in a script test `scripts/test-make.sh`)
- [ ] README documents layout

## Files in scope
`Makefile`, `.editorconfig`, `.gitignore`, `services/README.md`, `libs/README.md`, `contracts/README.md`, `deploy/README.md`, `scripts/test-make.sh`

## Depends on
none
