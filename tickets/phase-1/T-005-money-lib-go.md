---
id: T-005
title: "[T-005] Go money library with currency-aware rounding"
labels: phase:1, status:needs-approval, lang:go, area:pricing, size:S
---
## Why
PRC-1: all amounts in minor units with correct rounding; foundation for pricing. SPEC-05.

## What
Create Go module `libs/go-money` with type `Money{Amount int64, Currency string}`, currency exponent table for at least USD, EUR, GBP, JPY (0 decimals), KWD (3 decimals), operations Add, Sub (same currency only), MulRatio(num,den) with half-even rounding, Allocate(n parts, no cent lost), comparison, JSON marshal.

## Constraints
Go 1.23, standard library plus only dependencies named here; `gofmt` and `golangci-lint` clean. No floating point anywhere in the package (lint rule or test that greps).

## Out of scope
Currency conversion, formatting for display, price lists.

## Acceptance criteria (validations)
- [ ] Add/Sub across different currencies returns an error
- [ ] MulRatio rounds half-even: table-driven tests including .5 boundaries for JPY and KWD
- [ ] Allocate(100,[1,1,1]) = [34,33,33] and sum is always preserved (property test)
- [ ] Overflow on Add returns an error, never wraps
- [ ] Coverage >= 90% on package

## Files in scope
`libs/go-money/**`

## Depends on
T-001
