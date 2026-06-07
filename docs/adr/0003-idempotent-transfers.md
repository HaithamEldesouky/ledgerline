# ADR-0003: Idempotent transfers via an idempotency key

- **Status:** Accepted
- **Date:** 2026-06-07

## Context

Network clients retry. Without protection, a retried transfer request could
apply the same posting twice, double-spending or double-crediting an account —
a classic payments hazard.

## Decision

Support an optional `Idempotency-Key` HTTP header on `POST /api/v1/transfers`.
The key is persisted with the transfer:

- A fast path checks for an existing transfer with the key and returns it.
- The check is repeated inside the atomic `ExecuteTransfer` critical section to
  defeat concurrent duplicates.
- In PostgreSQL the key is backed by a **unique index**, so duplicates are safe
  even across multiple replicas; a unique-violation race re-reads and returns
  the winning transfer.

## Consequences

- Clients can retry transfers safely; a repeated key returns the original
  result without re-applying it.
- Keys must be unique per logical operation; reusing a key for a different
  transfer intentionally returns the first transfer.
- A small amount of extra storage and an index are required.
