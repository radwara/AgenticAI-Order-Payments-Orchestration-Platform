# SPEC-05 Pricing Service (Go)
**PRD refs:** §5.4 PRC-1..3 · **Tickets:** T-005, T-006

- Money type in minor units with currency exponent table; rounding rule per currency (half-even default).
- Quote = f(line items, price list version, promotion rules version). Same inputs and versions always return the same quote and a quote hash.
