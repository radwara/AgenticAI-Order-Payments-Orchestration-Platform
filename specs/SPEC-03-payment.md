# SPEC-03 Payment Service (Java 21)
**PRD refs:** §5.2 PAY-1, PAY-2, PAY-4 (PAY-3, 5, 6, 7 in later phases) · **Tickets:** T-015, T-016, T-017

- Provider adapter interface: authorize, capture, void, refund (full/partial). Mock provider with deterministic outcomes selected by test amounts/tokens.
- Only provider tokens handled; never raw card data.
- Double-entry ledger: each money movement posts balanced entries (sum of debits equals sum of credits per currency) in one DB transaction; ledger rows are append-only.
