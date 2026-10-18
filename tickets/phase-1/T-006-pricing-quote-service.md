---
id: T-006
title: "[T-006] Pricing quote endpoint (deterministic)"
labels: phase:1, status:needs-approval, lang:go, area:pricing, size:M
---
## Why
PRC-2, PRC-3: quotes must be reproducible for audit and retry. SPEC-05.

## What
Create `services/pricing` (Go) exposing gRPC `Quote(QuoteRequest) returns QuoteResponse` per `contracts/pricing/v1/pricing.proto`: line items (sku, qty, unit price minor units, currency), price list version, promotion rule version. Response: line totals, discount lines, total, and `quote_hash` (SHA-256 over canonical request + versions). Load price lists and promo rules (percentage-off, fixed-off, per SKU) from versioned JSON files in `services/pricing/testdata` in v1; unknown version returns `NOT_FOUND`.

## Constraints
Go 1.23, standard library plus only dependencies named here; `gofmt` and `golangci-lint` clean. Uses `libs/go-money`. gRPC via `google.golang.org/grpc` and `google.golang.org/protobuf` only. No clock, randomness or map-iteration-order dependence in the calculation path.

## Out of scope
Database-backed price lists, caching, admin APIs, tax.

## Acceptance criteria (validations)
- [ ] Same request and versions called 1000 times returns identical bytes (test)
- [ ] Reordering line items in the request yields the same total and the same hash (canonicalisation test)
- [ ] Percentage discount rounds half-even per currency (JPY and USD cases)
- [ ] Unknown price list version -> NOT_FOUND; negative quantity -> INVALID_ARGUMENT
- [ ] `/healthz` and `/readyz` HTTP endpoints return 200

## Files in scope
`services/pricing/**`, `contracts/pricing/**`

## Depends on
T-005
