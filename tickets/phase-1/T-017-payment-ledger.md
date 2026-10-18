---
id: T-017
title: "[T-017] Double-entry ledger"
labels: phase:1, status:needs-approval, lang:java, area:payment, size:M
---
## Why
PAY-4: every money movement balanced and auditable; duplicate-charge metric depends on it. SPEC-03.

## What
Add `ledger_entries(id, txn_id, account, direction DEBIT|CREDIT, amount_minor, currency, payment_id, created_at)` append-only, and `LedgerService.post(txn)`. Capture posts DEBIT customer_receivable / CREDIT merchant_payable; refund posts the reverse; void posts nothing. A transaction must balance per currency or `post` throws. Hook into capture and refund in the same DB transaction as the payment state change.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. No UPDATE or DELETE on ledger rows: enforce with a DB trigger raising an exception. No new dependencies.

## Out of scope
Reconciliation (PAY-7), multi-currency conversion, reporting.

## Acceptance criteria (validations)
- [ ] Unbalanced transaction is rejected and nothing is written (test)
- [ ] Capture followed by full refund nets to zero per account (test)
- [ ] Property test: random sequences of capture/partial refunds always leave the sum of debits equal to credits per currency
- [ ] UPDATE/DELETE on ledger_entries fails at DB level (integration test)
- [ ] Payment state change and ledger rows commit or roll back together (fault-injection test)

## Files in scope
`services/payment/**`

## Depends on
T-016
