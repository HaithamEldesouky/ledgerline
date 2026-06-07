-- ─────────────────────────────────────────────────────────────────────────
-- LedgerLine — initial schema
-- Double-entry ledger: accounts, transfers, and immutable ledger entries.
-- All monetary values are stored as BIGINT minor units (e.g. cents).
-- ─────────────────────────────────────────────────────────────────────────

BEGIN;

CREATE TABLE IF NOT EXISTS accounts (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    currency      CHAR(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    type          TEXT NOT NULL CHECK (type IN ('internal', 'external')),
    balance_minor BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Internal accounts may never hold a negative balance.
    CONSTRAINT internal_non_negative
        CHECK (type <> 'internal' OR balance_minor >= 0)
);

CREATE TABLE IF NOT EXISTS transfers (
    id               UUID PRIMARY KEY,
    from_account_id  UUID NOT NULL REFERENCES accounts (id),
    to_account_id    UUID NOT NULL REFERENCES accounts (id),
    amount_minor     BIGINT NOT NULL CHECK (amount_minor > 0),
    currency         CHAR(3) NOT NULL,
    idempotency_key  TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT different_accounts CHECK (from_account_id <> to_account_id)
);

-- Idempotency keys must be unique when present, making retries safe across
-- multiple service replicas.
CREATE UNIQUE INDEX IF NOT EXISTS transfers_idempotency_key_uniq
    ON transfers (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS ledger_entries (
    id                  UUID PRIMARY KEY,
    transfer_id         UUID NOT NULL REFERENCES transfers (id),
    account_id          UUID NOT NULL REFERENCES accounts (id),
    direction           TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    amount_minor        BIGINT NOT NULL CHECK (amount_minor > 0),
    balance_after_minor BIGINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ledger_entries_account_id_idx
    ON ledger_entries (account_id, created_at);

CREATE INDEX IF NOT EXISTS ledger_entries_transfer_id_idx
    ON ledger_entries (transfer_id);

COMMIT;
