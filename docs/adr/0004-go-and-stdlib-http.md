# ADR-0004: Go with the standard-library HTTP router

- **Status:** Accepted
- **Date:** 2026-06-07

## Context

The service is a cloud-native fintech microservice that must deploy as a small,
hardened container, start fast, handle concurrency safely, and sit naturally
alongside the surrounding infrastructure (Kubernetes, Prometheus, Terraform).

## Decision

Implement the service in **Go**, using the standard library's `net/http` with
the method-aware routing introduced in Go 1.22 (e.g. `POST /api/v1/accounts`,
`GET /api/v1/accounts/{id}`). Dependencies are limited to well-established,
focused libraries: `pgx` (Postgres), `prometheus/client_golang` (metrics),
`google/uuid`, and `golang.org/x/time/rate`.

## Rationale

- **Operability** — Go compiles to a single static binary, enabling a
  `distroless/static` non-root image of a few MB and near-instant cold starts.
- **Concurrency & correctness** — goroutines and the race detector make
  concurrent transfer handling safe and testable.
- **Ecosystem fit** — the surrounding cloud-native toolchain is itself written
  in Go, so one toolchain spans the whole platform.
- **Minimal dependencies** — using stdlib routing avoids a web-framework
  dependency and keeps the surface small and stable.

## Consequences

- Some conveniences (declarative validation, auto-generated docs) are
  implemented explicitly; this is a deliberate trade for fewer dependencies.
- The OpenAPI document is maintained alongside the handlers and served via
  embedded Swagger UI.
