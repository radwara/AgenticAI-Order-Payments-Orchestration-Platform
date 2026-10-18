---
id: T-022
title: "[T-022] CI guard rails: coverage, dependency diff, CodeQL, branch protection"
labels: phase:1, status:needs-approval, area:platform, size:M
---
## Why
agents/guardrails.md promises mechanical gates; without them agents are trusted instead of checked. SPEC-06.

## What
Add to `.github/workflows` and `scripts/`: (1) coverage gate: changed lines >= 80% (Go via cover profile + `diff-cover`, Java via JaCoCo + `diff-cover`); (2) dependency-diff check `scripts/check-deps.sh`: fails when `go.mod`, `go.sum`, `pom.xml` gain a coordinate not mentioned in the linked ticket's Constraints; (3) diff-size limit 600 lines excluding lockfiles and generated files; (4) CodeQL for Go and Java; (5) `.github/dependabot.yml` for gomod, maven, github-actions; (6) `scripts/apply-branch-protection.sh` (run by a human admin) that protects `main`: PR required, 1 human approval, required status checks, no force-push, no direct push, dismiss stale approvals.

## Constraints
Only first-party GitHub actions, `github/codeql-action`, and `diff-cover` (pip, pinned). Scripts in bash/python3 stdlib. Branch protection script must be idempotent and must NOT run automatically in CI.

## Out of scope
Language build/test jobs (T-002), secrets scan (exists), SAST beyond CodeQL.

## Acceptance criteria (validations)
- [ ] Script tests prove: dependency added but not in ticket -> fail; dependency named in ticket -> pass
- [ ] Diff of 601 changed lines -> fail; 600 -> pass; lockfile lines excluded
- [ ] Coverage gate fails on a fixture diff with an untested changed line and passes when covered
- [ ] Workflows pass `actionlint`
- [ ] Branch protection script run with `--dry-run` prints the exact API payload and is idempotent

## Files in scope
`.github/workflows/**`, `.github/dependabot.yml`, `scripts/check-deps.sh`, `scripts/apply-branch-protection.sh`, `scripts/test-*.sh`

## Depends on
T-002
