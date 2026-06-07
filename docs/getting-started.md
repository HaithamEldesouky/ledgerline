# Getting Started (Plain English)

This page explains, in simple terms, **what LedgerLine is** and **how to run
it**. No prior knowledge needed.

## What does this software do?

LedgerLine is a small web service that keeps track of money — like the
bookkeeping engine behind a digital wallet or a banking app.

It can:

- **Create accounts** (for example, "Alice's wallet").
- **Move money** from one account to another.
- **Show an account's balance** and its full **history** of money in and out.

It does this using **double-entry bookkeeping**, the same method real banks and
accountants use: every time money moves, it is *taken out* of one account and
*put into* another by the exact same amount. Money is never created or lost by
accident — it always adds up.

A few things it is careful about:

- **No rounding mistakes.** Amounts are counted in the smallest unit of the
  currency (cents), so `100` means **$1.00**. There are no fractions to round.
- **Safe retries.** If the network hiccups and the same "move money" request is
  sent twice, it only happens once (you send an `Idempotency-Key` with the
  request).
- **No overdrawing.** A normal account can't go below zero.

It is a **backend service** — it talks JSON over HTTP. It has no fancy website;
instead it comes with a built-in API explorer page you can click around in.

## What you need

Pick **one** of these:

- **Option A — Go only:** install [Go](https://go.dev/dl/). Fastest way to try it.
- **Option B — Docker:** install [Docker](https://www.docker.com/). Runs the
  full setup (service + database + dashboards) with one command.

## How to run it

### Option A — Run with Go (simplest)

From the project folder:

```bash
go run ./cmd/ledgerline
```

That's it. The service starts on **http://localhost:8080** and stores data in
memory (no database needed). Stop it with `Ctrl+C`.

### Option B — Run everything with Docker

From the project folder:

```bash
docker compose up --build
```

This starts the service **plus** a PostgreSQL database, Prometheus (metrics),
and Grafana (dashboards). Stop it with `Ctrl+C`, or fully clean up with:

```bash
docker compose down -v
```

## Check it's working

Open the interactive API page in your browser:

> **http://localhost:8080/docs**

Or check its health from a terminal:

```bash
curl http://localhost:8080/health/ready
# {"status":"ready"}
```

## Try it out (move some money)

Copy-paste this into a terminal while the service is running. It creates a
funding account and a wallet, then puts $100.00 into the wallet:

```bash
BASE=http://localhost:8080

# 1) Create a funding source (an "external" account).
TREASURY=$(curl -s -X POST $BASE/api/v1/accounts \
  -d '{"name":"Treasury","currency":"USD","type":"external"}' | jq -r .id)

# 2) Create Alice's wallet (a normal "internal" account).
ALICE=$(curl -s -X POST $BASE/api/v1/accounts \
  -d '{"name":"Alice","currency":"USD","type":"internal"}' | jq -r .id)

# 3) Move $100.00 (10000 cents) from the treasury into Alice's wallet.
curl -s -X POST $BASE/api/v1/transfers \
  -H "Idempotency-Key: my-first-transfer" \
  -d "{\"fromAccountId\":\"$TREASURY\",\"toAccountId\":\"$ALICE\",\"amountMinor\":10000,\"currency\":\"USD\"}"

# 4) See Alice's balance (should be 10000 = $100.00).
curl -s $BASE/api/v1/accounts/$ALICE

# 5) See Alice's history.
curl -s $BASE/api/v1/accounts/$ALICE/transactions
```

(`jq` is a small tool for reading JSON. If you don't have it, you can still run
the requests — you'll just see the raw JSON responses.)

## Common settings

You can change behaviour with environment variables (see
[`.env.example`](../.env.example)). The most useful ones:

| Variable         | What it does                          | Default   |
| ---------------- | ------------------------------------- | --------- |
| `PORT`           | Which port to listen on               | `8080`    |
| `STORAGE_DRIVER` | `memory` (no DB) or `postgres`        | `memory`  |
| `DATABASE_URL`   | Database address (only for postgres)  | —         |
| `LOG_LEVEL`      | `debug` / `info` / `warn` / `error`   | `info`    |

Example — run on a different port:

```bash
PORT=9090 go run ./cmd/ledgerline
```

## Where to go next

- [README](../README.md) — full feature list and deployment options
- [API reference](api.md) — every endpoint in detail
- [Architecture](architecture.md) — how it's built inside
