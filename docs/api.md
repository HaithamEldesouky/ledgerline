# API Reference

Base URL (local): `http://localhost:8080`

Interactive docs (Swagger UI): `GET /docs`
OpenAPI document: `GET /openapi.yaml`

All monetary amounts are integers in **minor units** (e.g. cents): `100` USD
means $1.00.

## Conventions

- Request and response bodies are JSON.
- Errors share a common envelope:

  ```json
  {
    "error": {
      "code": "INSUFFICIENT_FUNDS",
      "message": "account \"...\" has insufficient funds for this transfer",
      "details": { "accountId": "...", "balanceMinor": 0, "amountMinor": 100 }
    }
  }
  ```

| Code                     | HTTP | Meaning                                  |
| ------------------------ | ---- | ---------------------------------------- |
| `VALIDATION_ERROR`       | 400  | Malformed or missing input               |
| `INVALID_AMOUNT`         | 400  | Amount is not a positive integer         |
| `SAME_ACCOUNT_TRANSFER`  | 400  | Source and destination are identical     |
| `ACCOUNT_NOT_FOUND`      | 404  | Referenced account does not exist        |
| `CURRENCY_MISMATCH`      | 409  | Transfer currency ≠ account currency     |
| `INSUFFICIENT_FUNDS`     | 422  | Internal account would go negative       |
| `RATE_LIMITED`           | 429  | Too many requests                        |
| `INTERNAL_SERVER_ERROR`  | 500  | Unexpected error                         |

## Endpoints

### Create an account

```http
POST /api/v1/accounts
Content-Type: application/json

{ "name": "Alice Wallet", "currency": "USD", "type": "internal" }
```

`type` is optional and defaults to `internal`. Returns `201` with the account.

### Get an account

```http
GET /api/v1/accounts/{id}
```

### List accounts

```http
GET /api/v1/accounts
```

### List an account's ledger entries

```http
GET /api/v1/accounts/{id}/transactions
```

### Create a transfer

```http
POST /api/v1/transfers
Content-Type: application/json
Idempotency-Key: 1c1f9b8e-...

{
  "fromAccountId": "…",
  "toAccountId": "…",
  "amountMinor": 10000,
  "currency": "USD"
}
```

Atomically debits `fromAccountId` and credits `toAccountId`. Returns `201` with
the transfer and its two ledger entries.

The optional `Idempotency-Key` header makes retries safe: replaying the same key
returns the original transfer without applying it again.

## Observability

| Endpoint         | Description                          |
| ---------------- | ------------------------------------ |
| `GET /health/live`  | Liveness probe                    |
| `GET /health/ready` | Readiness probe (checks storage)  |
| `GET /metrics`      | Prometheus metrics                |

## Worked example

```bash
BASE=http://localhost:8080

# Create a funding (external) account and a customer (internal) account.
TREASURY=$(curl -s -X POST $BASE/api/v1/accounts \
  -d '{"name":"Treasury","currency":"USD","type":"external"}' | jq -r .id)
ALICE=$(curl -s -X POST $BASE/api/v1/accounts \
  -d '{"name":"Alice","currency":"USD","type":"internal"}' | jq -r .id)

# Fund Alice with $100.00, safely retryable via the idempotency key.
curl -s -X POST $BASE/api/v1/transfers \
  -H "Idempotency-Key: demo-001" \
  -d "{\"fromAccountId\":\"$TREASURY\",\"toAccountId\":\"$ALICE\",\"amountMinor\":10000,\"currency\":\"USD\"}" | jq

# Inspect Alice's balance and history.
curl -s $BASE/api/v1/accounts/$ALICE | jq
curl -s $BASE/api/v1/accounts/$ALICE/transactions | jq
```
