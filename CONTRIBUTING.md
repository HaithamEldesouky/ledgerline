# Contributing to LedgerLine

Thanks for your interest in contributing! This document explains how to get set
up and the conventions the project follows.

## Prerequisites

- [Go](https://go.dev/dl/) (the version pinned in [`go.mod`](go.mod))
- [Docker](https://www.docker.com/) (optional, for the full local stack)
- [Make](https://www.gnu.org/software/make/) (optional, for the shortcuts below)

## Getting started

```bash
git clone https://github.com/haithamEldesouky/ledgerline.git
cd ledgerline
go mod download
make run          # starts the API on :8080 with the in-memory store
```

Open <http://localhost:8080/docs> for the interactive API reference.

## Development workflow

| Task                | Command          |
| ------------------- | ---------------- |
| Run the service     | `make run`       |
| Run tests           | `make test`      |
| Tests with coverage | `make cover`     |
| Format code         | `make fmt`       |
| Vet                 | `make vet`       |
| Lint                | `make lint`      |
| Everything CI runs  | `make ci`        |

Please make sure `make ci` passes before opening a pull request.

## Coding standards

- Code must be formatted with `gofmt -s` and pass `go vet`.
- Keep the domain layer (`internal/domain`) free of transport and storage
  concerns — see the [architecture guide](docs/architecture.md).
- Add or update tests for any behavioural change.
- Money is always represented as integer **minor units** — never floats.

## Commit messages

This project follows [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add multi-currency conversion endpoint
fix: reject transfers with zero amount
docs: clarify idempotency semantics
```

## Pull requests

1. Fork the repo and create a feature branch.
2. Make your change with tests and docs.
3. Ensure `make ci` is green.
4. Open a PR describing the change and the motivation.

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
