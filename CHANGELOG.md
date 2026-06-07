# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-06-07

### Added

- Double-entry ledger domain: accounts, transfers and immutable ledger entries.
- REST API: create/list/get accounts, list account entries, create transfers.
- Idempotent transfers via the `Idempotency-Key` header.
- Two storage adapters: in-memory and PostgreSQL (atomic, row-locked transfers).
- Observability: Prometheus metrics, structured `slog` logging, health probes.
- Per-IP rate limiting, security headers and strict request validation.
- OpenAPI 3 specification served with embedded Swagger UI at `/docs`.
- Deployment tooling: multi-stage distroless Dockerfile, Docker Compose stack,
  Kubernetes manifests, Helm chart and Terraform for AWS.
- CI/CD: lint, unit + integration tests, race detector, vulnerability scanning,
  and container image build/push to GHCR.
