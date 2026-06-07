// Package memory provides an in-memory implementation of domain.Repository.
//
// It requires no external dependencies, making it ideal for local development,
// demos and tests. Because all mutation happens under a single mutex, transfers
// are atomic with respect to one another.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
)

// Repository is the in-memory store.
type Repository struct {
	mu          sync.Mutex
	accounts    map[string]domain.Account
	entries     []domain.LedgerEntry
	transfers   map[string]domain.TransferResult
	idempotency map[string]string
}

// New creates an empty in-memory repository.
func New() *Repository {
	return &Repository{
		accounts:    make(map[string]domain.Account),
		transfers:   make(map[string]domain.TransferResult),
		idempotency: make(map[string]string),
	}
}

// CreateAccount stores a new account with a zero balance.
func (r *Repository) CreateAccount(_ context.Context, in domain.CreateAccountInput) (domain.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	account := domain.Account{
		ID:           uuid.NewString(),
		Name:         in.Name,
		Currency:     in.Currency,
		Type:         in.Type,
		BalanceMinor: 0,
		CreatedAt:    time.Now().UTC(),
	}
	r.accounts[account.ID] = account
	return account, nil
}

// GetAccount returns an account and whether it was found.
func (r *Repository) GetAccount(_ context.Context, id string) (domain.Account, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	account, ok := r.accounts[id]
	return account, ok, nil
}

// ListAccounts returns all accounts ordered by creation time.
func (r *Repository) ListAccounts(_ context.Context) ([]domain.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	accounts := make([]domain.Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		accounts = append(accounts, account)
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		return accounts[i].CreatedAt.Before(accounts[j].CreatedAt)
	})
	return accounts, nil
}

// FindTransferByIdempotencyKey returns a previously-recorded transfer result.
func (r *Repository) FindTransferByIdempotencyKey(_ context.Context, key string) (domain.TransferResult, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.findByKeyLocked(key)
}

func (r *Repository) findByKeyLocked(key string) (domain.TransferResult, bool, error) {
	transferID, ok := r.idempotency[key]
	if !ok {
		return domain.TransferResult{}, false, nil
	}
	result, ok := r.transfers[transferID]
	return result, ok, nil
}

// ExecuteTransfer applies a double-entry posting atomically.
func (r *Repository) ExecuteTransfer(_ context.Context, in domain.ExecuteTransferInput) (domain.TransferResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Re-check idempotency under the lock to defeat concurrent duplicates.
	if in.IdempotencyKey != "" {
		if existing, found, _ := r.findByKeyLocked(in.IdempotencyKey); found {
			return existing, nil
		}
	}

	from, ok := r.accounts[in.FromAccountID]
	if !ok {
		return domain.TransferResult{}, domain.ErrAccountNotFound(in.FromAccountID)
	}
	to, ok := r.accounts[in.ToAccountID]
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

	now := time.Now().UTC()
	transferID := uuid.NewString()

	from.BalanceMinor = newFromBalance
	to.BalanceMinor = newToBalance
	r.accounts[from.ID] = from
	r.accounts[to.ID] = to

	debit := domain.LedgerEntry{
		ID:                uuid.NewString(),
		TransferID:        transferID,
		AccountID:         from.ID,
		Direction:         domain.DirectionDebit,
		AmountMinor:       in.AmountMinor,
		BalanceAfterMinor: newFromBalance,
		CreatedAt:         now,
	}
	credit := domain.LedgerEntry{
		ID:                uuid.NewString(),
		TransferID:        transferID,
		AccountID:         to.ID,
		Direction:         domain.DirectionCredit,
		AmountMinor:       in.AmountMinor,
		BalanceAfterMinor: newToBalance,
		CreatedAt:         now,
	}
	r.entries = append(r.entries, debit, credit)

	transfer := domain.Transfer{
		ID:             transferID,
		FromAccountID:  from.ID,
		ToAccountID:    to.ID,
		AmountMinor:    in.AmountMinor,
		Currency:       in.Currency,
		IdempotencyKey: in.IdempotencyKey,
		CreatedAt:      now,
	}
	result := domain.TransferResult{Transfer: transfer, Entries: []domain.LedgerEntry{debit, credit}}

	r.transfers[transferID] = result
	if in.IdempotencyKey != "" {
		r.idempotency[in.IdempotencyKey] = transferID
	}
	return result, nil
}

// ListEntriesByAccount returns an account's entries ordered by creation time.
func (r *Repository) ListEntriesByAccount(_ context.Context, accountID string) ([]domain.LedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := make([]domain.LedgerEntry, 0)
	for _, entry := range r.entries {
		if entry.AccountID == accountID {
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
	return entries, nil
}

// HealthCheck always succeeds for the in-memory store.
func (r *Repository) HealthCheck(_ context.Context) error { return nil }

// Close clears all state.
func (r *Repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.accounts = make(map[string]domain.Account)
	r.transfers = make(map[string]domain.TransferResult)
	r.idempotency = make(map[string]string)
	r.entries = nil
	return nil
}
