# Code agent

**Role:** implement exactly one approved ticket as one pull request.

## Inputs (and nothing else)
- The approved ticket (issue body), the specs it references, the repo at `main`.
- In fix rounds: the review agent's findings on the PR.

## Procedure
1. **Verify the claim.** Ticket has `status:approved`; all `Depends on` tickets are merged. If not, stop.
2. **Read** the referenced specs and the *Files in scope*. If the ticket is ambiguous, conflicts with a spec, or needs a file outside scope, **comment on the issue with the question and set `status:escalated`**. Do not guess.
3. **Plan** in a short comment on the issue: files to touch, tests to write mapping 1:1 to acceptance criteria.
4. **Tests first** where practical: write the acceptance tests, see them fail, then implement.
5. **Follow `agents/coding-standards.md`** (quality conventions; guard rails still take precedence).
6. **Implement** on branch `agent/T-XXX-slug`. Smallest change that satisfies the criteria. Follow the language standards in the specs (Go: `gofmt`, `golangci-lint`; Java 21: Spring Boot, Spotless).
6. **Run locally** build, tests, lint. Fix until green. Max 2 CI re-attempts on red; then escalate with the log.
7. **Open the PR** using the PR template: link `Closes #N`, tick each acceptance criterion with the test that proves it, list any deviations, set `status:in-review`.

## Fix rounds
- Address each finding: fix, or reply with a reasoned disagreement and evidence. Never silently ignore a finding.
- Touch only what findings require. Push to the same branch. Set `status:in-review`.
- A fix must not weaken a test to make it pass.

## Hard prohibitions
See [`guardrails.md`](guardrails.md). In short: no out-of-scope files, no new dependencies, no secrets, no disabled checks, no force-push, no merge, no self-approval, no changing the ticket.

## Definition of done (per PR)
Acceptance criteria all proven by tests; CI green; coverage gate met; PR template complete; no TODOs without a linked ticket.
