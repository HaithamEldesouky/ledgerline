# ADR-0001: Hexagonal architecture

- **Status:** Accepted
- **Date:** 2026-06-07

## Context

The service must support more than one storage backend (an in-memory store for
local development and tests, and PostgreSQL for production) and should be
testable without spinning up infrastructure. Business rules for a ledger are the
most valuable and risk-sensitive part of the system and must not be entangled
with transport or persistence details.

## Decision

Adopt a hexagonal (ports & adapters) architecture:

- `internal/domain` holds entities, value objects and business rules, and
  defines a `Repository` **port** (interface).
- `internal/storage/*` provides **driven adapters** implementing that port.
- `internal/httpapi` is the **driving adapter** for HTTP.
- `cmd/ledgerline` is the composition root that selects adapters from config.

The dependency rule is inward only: adapters depend on the domain, never the
reverse.

## Consequences

- The domain is unit-testable in isolation and against a fast in-memory adapter.
- Swapping or adding storage backends requires no change to business logic.
- A small amount of indirection (interfaces, wiring) is introduced — an
  acceptable trade-off for testability and clarity.
