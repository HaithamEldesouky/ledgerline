# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability, please **do not** open a public issue.
Instead, report it privately via GitHub's
[security advisories](https://github.com/haithamEldesouky/ledgerline/security/advisories/new)
so it can be addressed before public disclosure.

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce or a proof of concept
- Any suggested remediation

You can expect an initial acknowledgement within a few business days.

## Security posture

LedgerLine is built with security as a first-class concern:

- **Minimal runtime image** — the container is built `FROM` a distroless
  static base with no shell or package manager, and runs as a non-root user.
- **Hardened Kubernetes manifests** — `readOnlyRootFilesystem`, dropped
  capabilities, `runAsNonRoot`, and a `RuntimeDefault` seccomp profile.
- **Dependency scanning** — Dependabot keeps Go modules, GitHub Actions and the
  base image up to date; CI runs a vulnerability scan on every change.
- **Input validation** — all request bodies are strictly validated and request
  sizes are bounded.
- **Money safety** — amounts are integer minor units to eliminate
  floating-point rounding errors.
