// Package postgres provides a durable, PostgreSQL-backed implementation of
// domain.Repository using the pgx driver.
//
// Transfers run inside a single SQL transaction. Both account rows are locked
// with SELECT ... FOR UPDATE in a deterministic order to avoid deadlocks, so
// balance checks and mutations are serialised at the database level. The
// idempotency key is protected by a unique index, making duplicate
// submissions safe even across multiple service replicas.
package postgres

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
)

// uniqueViolation is the PostgreSQL SQLSTATE for a unique-constraint breach.
const uniqueViolation = "23505"

// Repository is the PostgreSQL store.
type Repository struct {
	pool *pgxpool.Pool
}

// New connects to PostgreSQL and verifies the connection.
func New(ctx context.Context, connString string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Repository{pool: pool}, nil
}

// CreateAccount inserts a new account with a zero balance.
func (r *Repository) CreateAccount(ctx context.Context, in domain.CreateAccountInput) (domain.Account, error) {
	id := uuid.NewString()
	row := r.pool.QueryRow(ctx,
		`INSERT INTO accounts (id, name, currency, type, balance_minor)
		 VALUES ($1, $2, $3, $4, 0)
		 RETURNING id, name, currency, type, balance_minor, created_at`,
		id, in.Name, in.Currency, string(in.Type),
	)
	return scanAccount(row)
}

// GetAccount returns an account and whether it was found.
func (r *Repository) GetAccount(ctx context.Context, id string) (domain.Account, bool, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, currency, type, balance_minor, created_at
		 FROM accounts WHERE id = $1`, id)
	account, err := scanAccount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	return account, true, nil
}

// ListAccounts returns all accounts ordered by creation time.
func (r *Repository) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, currency, type, balance_minor, created_at
		 FROM accounts ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]domain.Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

// FindTransferByIdempotencyKey returns a previously-recorded transfer result.
func (r *Repository) FindTransferByIdempotencyKey(ctx context.Context, key string) (domain.TransferResult, bool, error) {
	return findTransferByKey(ctx, r.pool, key)
}

// ExecuteTransfer applies a double-entry posting atomically.
func (r *Repository) ExecuteTransfer(ctx context.Context, in domain.ExecuteTransferInput) (domain.TransferResult, error) {
	result, err := r.executeTransferTx(ctx, in)
	// Handle a cross-replica idempotency race: another transaction committed
	// the same key first. Re-read and return the winning result.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && in.IdempotencyKey != "" {
		if existing, found, ferr := findTransferByKey(ctx, r.pool, in.IdempotencyKey); ferr == nil && found {
			return existing, nil
		}
	}
	return result, err
}

func (r *Repository) executeTransferTx(ctx context.Context, in domain.ExecuteTransferInput) (domain.TransferResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.TransferResult{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if in.IdempotencyKey != "" {
		if existing, found, ferr := findTransferByKeyTx(ctx, tx, in.IdempotencyKey); ferr != nil {
			return domain.TransferResult{}, ferr
		} else if found {
			if err := tx.Commit(ctx); err != nil {
				return domain.TransferResult{}, err
			}
			committed = true
			return existing, nil
		}
	}

	// Lock both rows in a stable order to prevent deadlocks.
	ids := []string{in.FromAccountID, in.ToAccountID}
	sort.Strings(ids)
	rows, err := tx.Query(ctx,
		`SELECT id, name, currency, type, balance_minor, created_at
		 FROM accounts WHERE id = ANY($1::uuid[]) FOR UPDATE`, ids)
	if err != nil {
		return domain.TransferResult{}, err
	}
	locked := make(map[string]domain.Account)
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			rows.Close()
			return domain.TransferResult{}, scanErr
		}
		locked[account.ID] = account
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return domain.TransferResult{}, err
	}

	from, ok := locked[in.FromAccountID]
	if !ok {
		return domain.TransferResult{}, domain.ErrAccountNotFound(in.FromAccountID)
	}
	to, ok := locked[in.ToAccountID]
	if !ok {
		return domain.TransferResult{}, domain.ErrAccountNotFound(in.ToAccountID)
	}

	if from.Currency != in.Currency {
		return domain.TransferResult{}, domain.ErrCurrencyMismatch(in.Currency, from.Currency)
	}
	if to.Currency != in.Currency {
		return domain.TransferResult{}, domain.ErrCurrencyMismatch(in.Currency, to.Currency)
	}

	newFromBalance := from.BalanceMinor - in.AmountMinor
	if from.Type == domain.AccountInternal && newFromBalance < 0 {
		return domain.TransferResult{}, domain.ErrInsufficientFunds(from.ID, from.BalanceMinor, in.AmountMinor)
	}
	newToBalance := to.BalanceMinor + in.AmountMinor

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_minor = $1 WHERE id = $2`, newFromBalance, from.ID); err != nil {
		return domain.TransferResult{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_minor = $1 WHERE id = $2`, newToBalance, to.ID); err != nil {
		return domain.TransferResult{}, err
	}

	transferID := uuid.NewString()
	var idempotencyArg any
	if in.IdempotencyKey != "" {
		idempotencyArg = in.IdempotencyKey
	}
	transferRow := tx.QueryRow(ctx,
		`INSERT INTO transfers
		   (id, from_account_id, to_account_id, amount_minor, currency, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, from_account_id, to_account_id, amount_minor, currency,
		           idempotency_key, created_at`,
		transferID, from.ID, to.ID, in.AmountMinor, in.Currency, idempotencyArg,
	)
	transfer, err := scanTransfer(transferRow)
	if err != nil {
		return domain.TransferResult{}, err
	}

	debit, err := scanEntry(tx.QueryRow(ctx,
		`INSERT INTO ledger_entries
		   (id, transfer_id, account_id, direction, amount_minor, balance_after_minor)
		 VALUES ($1, $2, $3, 'debit', $4, $5)
		 RETURNING id, transfer_id, account_id, direction, amount_minor,
		           balance_after_minor, created_at`,
		uuid.NewString(), transferID, from.ID, in.AmountMinor, newFromBalance))
	if err != nil {
		return domain.TransferResult{}, err
	}
	credit, err := scanEntry(tx.QueryRow(ctx,
		`INSERT INTO ledger_entries
		   (id, transfer_id, account_id, direction, amount_minor, balance_after_minor)
		 VALUES ($1, $2, $3, 'credit', $4, $5)
		 RETURNING id, transfer_id, account_id, direction, amount_minor,
		           balance_after_minor, created_at`,
		uuid.NewString(), transferID, to.ID, in.AmountMinor, newToBalance))
	if err != nil {
		return domain.TransferResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TransferResult{}, err
	}
	committed = true

	return domain.TransferResult{
		Transfer: transfer,
		Entries:  []domain.LedgerEntry{debit, credit},
	}, nil
}

// ListEntriesByAccount returns an account's entries ordered by creation time.
func (r *Repository) ListEntriesByAccount(ctx context.Context, accountID string) ([]domain.LedgerEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, transfer_id, account_id, direction, amount_minor,
		        balance_after_minor, created_at
		 FROM ledger_entries WHERE account_id = $1 ORDER BY created_at ASC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]domain.LedgerEntry, 0)
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// HealthCheck pings the database.
func (r *Repository) HealthCheck(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// Close releases the connection pool.
func (r *Repository) Close() error {
	r.pool.Close()
	return nil
}

// --- scanning helpers ---

// scannable is satisfied by both pgx.Row and pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
}

func scanAccount(row scannable) (domain.Account, error) {
	var a domain.Account
	var accountType string
	err := row.Scan(&a.ID, &a.Name, &a.Currency, &accountType, &a.BalanceMinor, &a.CreatedAt)
	if err != nil {
		return domain.Account{}, err
	}
	a.Type = domain.AccountType(accountType)
	return a, nil
}

func scanEntry(row scannable) (domain.LedgerEntry, error) {
	var e domain.LedgerEntry
	var direction string
	err := row.Scan(&e.ID, &e.TransferID, &e.AccountID, &direction,
		&e.AmountMinor, &e.BalanceAfterMinor, &e.CreatedAt)
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	e.Direction = domain.EntryDirection(direction)
	return e, nil
}

func scanTransfer(row scannable) (domain.Transfer, error) {
	var t domain.Transfer
	var key *string
	err := row.Scan(&t.ID, &t.FromAccountID, &t.ToAccountID, &t.AmountMinor,
		&t.Currency, &key, &t.CreatedAt)
	if err != nil {
		return domain.Transfer{}, err
	}
	if key != nil {
		t.IdempotencyKey = *key
	}
	return t, nil
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func findTransferByKey(ctx context.Context, q querier, key string) (domain.TransferResult, bool, error) {
	transfer, err := scanTransfer(q.QueryRow(ctx,
		`SELECT id, from_account_id, to_account_id, amount_minor, currency,
		        idempotency_key, created_at
		 FROM transfers WHERE idempotency_key = $1`, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TransferResult{}, false, nil
	}
	if err != nil {
		return domain.TransferResult{}, false, err
	}

	rows, err := q.Query(ctx,
		`SELECT id, transfer_id, account_id, direction, amount_minor,
		        balance_after_minor, created_at
		 FROM ledger_entries WHERE transfer_id = $1 ORDER BY direction DESC`, transfer.ID)
	if err != nil {
		return domain.TransferResult{}, false, err
	}
	defer rows.Close()

	entries := make([]domain.LedgerEntry, 0, 2)
	for rows.Next() {
		entry, scanErr := scanEntry(rows)
		if scanErr != nil {
			return domain.TransferResult{}, false, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return domain.TransferResult{}, false, err
	}
	return domain.TransferResult{Transfer: transfer, Entries: entries}, true, nil
}

func findTransferByKeyTx(ctx context.Context, tx pgx.Tx, key string) (domain.TransferResult, bool, error) {
	return findTransferByKey(ctx, tx, key)
}
