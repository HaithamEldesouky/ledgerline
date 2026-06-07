# syntax=docker/dockerfile:1

# ─── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS build

# Build metadata, injected by CI (see .github/workflows).
ARG VERSION=dev

WORKDIR /src

# Download dependencies first so this layer is cached unless go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Build a fully static, stripped binary for a scratch/distroless runtime.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/ledgerline ./cmd/ledgerline

# ─── Runtime stage ────────────────────────────────────────────────────────────
# distroless/static contains only CA certificates, tzdata and a nonroot user —
# no shell, no package manager — minimising the attack surface.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=build /out/ledgerline /ledgerline

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/ledgerline"]
