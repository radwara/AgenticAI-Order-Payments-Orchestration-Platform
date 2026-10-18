---
id: T-015
title: "[T-015] Provider adapter interface and mock provider"
labels: phase:1, status:needs-approval, lang:java, area:payment, size:S
---
## Why
PAY-2: providers are pluggable; sandbox needs a deterministic mock. SPEC-03.

## What
Create `services/payment` module. Define `PaymentProvider` interface (authorize, capture, void, refund) using `Money` and an opaque provider token, returning a sealed result type (Success, Declined(reason), TransientError). Implement `MockProvider` where the outcome is chosen by token prefix: `tok_ok`, `tok_declined`, `tok_timeout` (TransientError), `tok_insufficient`.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Pure Java, no HTTP. Raw card fields (PAN, CVV) must not exist in any type; add an ArchUnit test banning fields named pan/cvv/cardNumber.

## Out of scope
Real providers, retries, circuit breaker, persistence.

## Acceptance criteria (validations)
- [ ] Each token prefix yields its documented result for all four operations
- [ ] Capture more than authorized -> Declined(AMOUNT_EXCEEDS_AUTH)
- [ ] Refund total never exceeds captured amount (test)
- [ ] ArchUnit test bans card-data field names
- [ ] Interface Javadoc lists idempotency expectations for implementers

## Files in scope
`services/payment/pom.xml`, `services/payment/src/**`

## Depends on
T-001
