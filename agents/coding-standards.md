# Coding standards (code agent and review agent)

**These are standards, not guard rails.** Guard rails (`guardrails.md`) are hard, mechanically enforced limits (scope, dependencies, secrets, CI gates, money/state invariants). Standards are quality conventions the code agent follows and the review agent checks. A standards violation is normally `minor`/`nit` (category `maintainability`); it becomes `major` only when it breaks a guard rail, a domain invariant, or obscures a correctness defect. Where a linter can enforce a rule, the linter is the enforcement and CI is the gate.

Adapted from the MIT-licensed "coding-standards" skill in affaan-m/ECC (pinned at commit 76aaa77df6adc5b889e689007bf99b9a4f74af34, `.agents/skills/coding-standards/SKILL.md`). That skill targets TypeScript/React/Next.js; only its language-agnostic principles are kept. Language-specific parts below are this project's own.

## Principles (all languages)
1. **Readability first.** Descriptive names; self-documenting code; comments explain *why*, not *what*.
2. **KISS.** Simplest thing that satisfies the ticket's acceptance criteria. No speculative abstraction.
3. **YAGNI.** Build only what the ticket asks for (also enforced by scope guard rails).
4. **DRY with judgement.** Extract shared logic once it repeats three times or must stay consistent (e.g. money rounding); do not couple unrelated code to avoid duplication.
5. **Small functions, early returns.** Aim for under about 50 lines and at most 3 levels of nesting; prefer guard clauses.
6. **No magic numbers or strings.** Name them (`MaxRetries`, `reservationTTL`), or take them from config.
7. **Explicit errors.** Never swallow an error; wrap with context; no panics/uncaught exceptions for expected failures; map errors to RFC 9457 problem responses at the API edge.
8. **Immutability by default.** Prefer immutable values/records/final fields; mutate only inside an aggregate or a transaction boundary.
9. **Tests.** Arrange-Act-Assert; descriptive names that state behaviour (`rejects_refund_above_captured_amount`); one behaviour per test; every acceptance criterion maps to a test; assertions must be able to fail.
10. **Public API documentation.** Doc comments on exported/public symbols state behaviour, errors and idempotency expectations.

## Go
- `gofmt`, `go vet`, `golangci-lint` clean (CI gate). Package names short and lower-case; exported names documented.
- `context.Context` first parameter for anything doing I/O; honour cancellation; every goroutine has an owner and a stop condition.
- Return errors with `%w`; check with `errors.Is/As`; no `panic` outside `main` initialisation.
- Table-driven tests, `-race`; inject clocks and randomness.
- Money via `libs/go-money` only (no floats).

## Java 21 / Spring Boot
- Spotless clean (CI gate). Constructor injection; no field injection; no `@Autowired` on fields.
- Records for DTOs and value objects; sealed types for closed result sets; `Optional` only as return type.
- Transactions at the service layer; controllers thin; domain logic in the domain package (no Spring in `domain`).
- Bean Validation at the API edge; JPA entities never exposed in API responses.
- JUnit 5 + AssertJ; Testcontainers for Postgres; ArchUnit for architectural rules.

## SQL and API
- Migrations are forward-only, reviewed, and run by the pipeline (not at startup). Select explicit columns. Keyset pagination, never OFFSET on large tables.
- API changes follow the OpenAPI/proto contract first; breaking changes need a ticket.

## What the review agent does with these
Flag violations with a concrete example and a suggested fix, severity `minor` or `nit`, category `maintainability`. Do not block a PR on style alone. Do not flag what the formatter or linter already enforces.
