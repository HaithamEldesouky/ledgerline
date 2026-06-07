# ADR-0002: Represent money as integer minor units

- **Status:** Accepted
- **Date:** 2026-06-07

## Context

The system moves money and must reconcile balances exactly. Floating-point
types (`float64`) cannot represent most decimal fractions (e.g. 0.10) exactly,
leading to rounding drift that is unacceptable in a financial ledger.

## Decision

Represent all monetary values as `int64` in **minor units** (e.g. cents).
`100` with currency `USD` means $1.00. Conversion to a human-readable decimal
string is a presentation concern handled at the edges (`FormatMinor`).

Currency is stored as an ISO 4217 three-letter code; transfers require both
accounts and the request to share the same currency.

## Consequences

- Arithmetic is exact; balances reconcile precisely.
- Clients must send and interpret amounts in minor units (clearly documented).
- Multi-currency conversion is out of scope and would require an explicit
  exchange-rate boundary if added later.
