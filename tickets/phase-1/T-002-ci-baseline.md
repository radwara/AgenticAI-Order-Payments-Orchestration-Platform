---
id: T-002
title: "[T-002] Path-filtered CI for Go and Java"
labels: phase:1, status:needs-approval, area:platform, size:S
---
## Why
Guard rails need mechanical gates (agents/guardrails.md). SPEC-01.

## What
Extend `.github/workflows/ci.yml` with Go and Java jobs triggered only when matching paths change: Go (`gofmt` check, `go vet`, `golangci-lint`, `go test -race -cover`), Java (`mvn -B verify` with Spotless check). Jobs are skipped cleanly when no module exists. Add a `scope-check` job running `scripts/check-scope.sh` that fails if the PR touches files outside the linked ticket's *Files in scope* (read via `gh issue view`). Add `scripts/check-scope.sh`.

## Constraints
Use only official `actions/*`, `actions/setup-go`, `actions/setup-java`, `golangci/golangci-lint-action`. Pin actions by major version at minimum. No secrets required.

## Out of scope
CodeQL, coverage gate, dependency-diff check (separate ticket), branch protection settings.

## Acceptance criteria (validations)
- [ ] Workflow YAML passes `actionlint`
- [ ] `scripts/check-scope.sh` has a bats/shell test with an in-scope and an out-of-scope file list
- [ ] On a PR with no Go/Java changes, language jobs are skipped and the workflow is green

## Files in scope
`.github/workflows/ci.yml`, `scripts/check-scope.sh`, `scripts/test-check-scope.sh`

## Depends on
T-001
