package domain

import (
	"context"
	"strings"
)

// CreateAccountInput is the persistence-level payload for a new account.
type CreateAccountInput struct {
	Name     string
	Currency string
	Type     AccountType
}

// ExecuteTransferInput is the persistence-level payload for a transfer.
type ExecuteTransferInput struct {
	FromAccountID  string
	ToAccountID    string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
}

// Repository is the persistence port (hexagonal "driven" port) for the ledger.
//
// Implementations own atomicity: ExecuteTransfer must apply both postings,
// update both balances, persist the transfer and honour the idempotency key as
// a single all-or-nothing operation, keeping the money-moving invariant safe
// under concurrency.
type Repository interface {
	CreateAccount(ctx context.Context, in CreateAccountInput) (Account, error)
	GetAccount(ctx context.Context, id string) (Account, bool, error)
	ListAccounts(ctx context.Context) ([]Account, error)
	FindTransferByIdempotencyKey(ctx context.Context, key string) (TransferResult, bool, error)
	ExecuteTransfer(ctx context.Context, in ExecuteTransferInput) (TransferResult, error)
	ListEntriesByAccount(ctx context.Context, accountID string) ([]LedgerEntry, error)
	// HealthCheck reports whether the backing store is reachable.
	HealthCheck(ctx context.Context) error
	// Close releases underlying resources (connection pools, etc.).
	Close() error
}

// CreateAccountCommand is the application-level request to create an account.
type CreateAccountCommand struct {
	Name     string
	Currency string
	Type     AccountType
}

// TransferCommand is the application-level request to move money.
type TransferCommand struct {
	FromAccountID  string
	ToAccountID    string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
}

// Service holds the transport-agnostic business rules of the ledger and
// delegates durable, atomic persistence to a Repository.
type Service struct {
	repo Repository
}

// NewService constructs a ledger Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateAccount creates a new account with a zero opening balance.
func (s *Service) CreateAccount(ctx context.Context, cmd CreateAccountCommand) (Account, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return Account{}, ErrValidation("account name must not be empty")
	}
	currency, err := NormalizeCurrency(cmd.Currency)
	if err != nil {
		return Account{}, err
	}
	accountType := cmd.Type
	if accountType == "" {
		accountType = AccountInternal
	}
	if accountType != AccountInternal && accountType != AccountExternal {
		return Account{}, ErrValidation("account type must be \"internal\" or \"external\"")
	}
	return s.repo.CreateAccount(ctx, CreateAccountInput{
		Name:     name,
		Currency: currency,
		Type:     accountType,
	})
}

// GetAccount returns a single account or ErrAccountNotFound.
func (s *Service) GetAccount(ctx context.Context, id string) (Account, error) {
	account, found, err := s.repo.GetAccount(ctx, id)
	if err != nil {
		return Account{}, err
	}
	if !found {
		return Account{}, ErrAccountNotFound(id)
	}
	return account, nil
}

// ListAccounts returns all accounts.
func (s *Service) ListAccounts(ctx context.Context) ([]Account, error) {
	return s.repo.ListAccounts(ctx)
}

// Transfer atomically moves money between two accounts using double-entry
// bookkeeping: the source is debited and the destination credited by the same
// amount, conserving the system-wide sum of balances.
//
// Cheap, store-free invariants are checked here; account existence, currency
// agreement and sufficient funds are enforced atomically inside the repository
// to remain race-safe.
func (s *Service) Transfer(ctx context.Context, cmd TransferCommand) (TransferResult, error) {
	currency, err := NormalizeCurrency(cmd.Currency)
	if err != nil {
		return TransferResult{}, err
	}
	if cmd.AmountMinor <= 0 {
		return TransferResult{}, ErrInvalidAmount(cmd.AmountMinor)
	}
	if cmd.FromAccountID == cmd.ToAccountID {
		return TransferResult{}, ErrSameAccountTransfer(cmd.FromAccountID)
	}

	// Fast path: return the previously-recorded result for a known key.
	if cmd.IdempotencyKey != "" {
		existing, found, ferr := s.repo.FindTransferByIdempotencyKey(ctx, cmd.IdempotencyKey)
		if ferr != nil {
			return TransferResult{}, ferr
		}
		if found {
			return existing, nil
		}
	}

	return s.repo.ExecuteTransfer(ctx, ExecuteTransferInput{
		FromAccountID:  cmd.FromAccountID,
		ToAccountID:    cmd.ToAccountID,
		AmountMinor:    cmd.AmountMinor,
		Currency:       currency,
		IdempotencyKey: cmd.IdempotencyKey,
	})
}

// GetAccountEntries returns the ordered ledger history for an account,
// surfacing a not-found error when the account does not exist.
func (s *Service) GetAccountEntries(ctx context.Context, accountID string) ([]LedgerEntry, error) {
	if _, err := s.GetAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return s.repo.ListEntriesByAccount(ctx, accountID)
}
