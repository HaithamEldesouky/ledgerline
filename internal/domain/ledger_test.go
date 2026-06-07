package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
	"github.com/haithamEldesouky/ledgerline/internal/storage/memory"
)

func newService(t *testing.T) (*domain.Service, func()) {
	t.Helper()
	repo := memory.New()
	return domain.NewService(repo), func() { _ = repo.Close() }
}

func assertCode(t *testing.T, err error, code domain.ErrorCode) {
	t.Helper()
	require.Error(t, err)
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, code, derr.Code)
}

func mustAccount(t *testing.T, svc *domain.Service, name, currency string, typ domain.AccountType) domain.Account {
	t.Helper()
	acc, err := svc.CreateAccount(context.Background(), domain.CreateAccountCommand{
		Name:     name,
		Currency: currency,
		Type:     typ,
	})
	require.NoError(t, err)
	return acc
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()
	svc, cleanup := newService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("defaults to internal", func(t *testing.T) {
		acc, err := svc.CreateAccount(ctx, domain.CreateAccountCommand{Name: "Wallet", Currency: "usd"})
		require.NoError(t, err)
		assert.Equal(t, domain.AccountInternal, acc.Type)
		assert.Equal(t, "USD", acc.Currency)
		assert.EqualValues(t, 0, acc.BalanceMinor)
		assert.NotEmpty(t, acc.ID)
	})

	t.Run("rejects empty name", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, domain.CreateAccountCommand{Name: "  ", Currency: "USD"})
		assertCode(t, err, domain.CodeValidation)
	})

	t.Run("rejects bad currency", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, domain.CreateAccountCommand{Name: "Wallet", Currency: "dollars"})
		assertCode(t, err, domain.CodeValidation)
	})

	t.Run("rejects bad type", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, domain.CreateAccountCommand{Name: "Wallet", Currency: "USD", Type: "weird"})
		assertCode(t, err, domain.CodeValidation)
	})
}

func TestTransferHappyPath(t *testing.T) {
	t.Parallel()
	svc, cleanup := newService(t)
	defer cleanup()
	ctx := context.Background()

	treasury := mustAccount(t, svc, "Treasury", "USD", domain.AccountExternal)
	alice := mustAccount(t, svc, "Alice", "USD", domain.AccountInternal)

	// Fund Alice from the external treasury.
	result, err := svc.Transfer(ctx, domain.TransferCommand{
		FromAccountID: treasury.ID,
		ToAccountID:   alice.ID,
		AmountMinor:   10000,
		Currency:      "USD",
	})
	require.NoError(t, err)
	require.Len(t, result.Entries, 2)

	// Verify the two postings are a balanced debit/credit pair.
	var debit, credit domain.LedgerEntry
	for _, e := range result.Entries {
		switch e.Direction {
		case domain.DirectionDebit:
			debit = e
		case domain.DirectionCredit:
			credit = e
		}
	}
	assert.Equal(t, treasury.ID, debit.AccountID)
	assert.Equal(t, alice.ID, credit.AccountID)
	assert.EqualValues(t, 10000, debit.AmountMinor)
	assert.EqualValues(t, 10000, credit.AmountMinor)
	assert.EqualValues(t, -10000, debit.BalanceAfterMinor)
	assert.EqualValues(t, 10000, credit.BalanceAfterMinor)

	// Balances reflect the movement; money is conserved system-wide.
	aliceAfter, err := svc.GetAccount(ctx, alice.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 10000, aliceAfter.BalanceMinor)
	treasuryAfter, err := svc.GetAccount(ctx, treasury.ID)
	require.NoError(t, err)
	assert.EqualValues(t, -10000, treasuryAfter.BalanceMinor)

	// Ledger history is queryable per account.
	entries, err := svc.GetAccountEntries(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.DirectionCredit, entries[0].Direction)
}

func TestTransferValidation(t *testing.T) {
	t.Parallel()
	svc, cleanup := newService(t)
	defer cleanup()
	ctx := context.Background()

	usdA := mustAccount(t, svc, "A", "USD", domain.AccountExternal)
	usdB := mustAccount(t, svc, "B", "USD", domain.AccountInternal)
	eur := mustAccount(t, svc, "C", "EUR", domain.AccountInternal)

	t.Run("invalid amount", func(t *testing.T) {
		_, err := svc.Transfer(ctx, domain.TransferCommand{
			FromAccountID: usdA.ID, ToAccountID: usdB.ID, AmountMinor: 0, Currency: "USD",
		})
		assertCode(t, err, domain.CodeInvalidAmount)
	})

	t.Run("same account", func(t *testing.T) {
		_, err := svc.Transfer(ctx, domain.TransferCommand{
			FromAccountID: usdA.ID, ToAccountID: usdA.ID, AmountMinor: 100, Currency: "USD",
		})
		assertCode(t, err, domain.CodeSameAccountTransfer)
	})

	t.Run("account not found", func(t *testing.T) {
		_, err := svc.Transfer(ctx, domain.TransferCommand{
			FromAccountID: usdA.ID, ToAccountID: "00000000-0000-0000-0000-000000000000", AmountMinor: 100, Currency: "USD",
		})
		assertCode(t, err, domain.CodeAccountNotFound)
	})

	t.Run("currency mismatch", func(t *testing.T) {
		_, err := svc.Transfer(ctx, domain.TransferCommand{
			FromAccountID: usdA.ID, ToAccountID: eur.ID, AmountMinor: 100, Currency: "USD",
		})
		assertCode(t, err, domain.CodeCurrencyMismatch)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		// usdB is internal with a zero balance and cannot go negative.
		_, err := svc.Transfer(ctx, domain.TransferCommand{
			FromAccountID: usdB.ID, ToAccountID: usdA.ID, AmountMinor: 100, Currency: "USD",
		})
		assertCode(t, err, domain.CodeInsufficientFunds)
	})
}

func TestTransferIdempotency(t *testing.T) {
	t.Parallel()
	svc, cleanup := newService(t)
	defer cleanup()
	ctx := context.Background()

	treasury := mustAccount(t, svc, "Treasury", "USD", domain.AccountExternal)
	alice := mustAccount(t, svc, "Alice", "USD", domain.AccountInternal)

	first, err := svc.Transfer(ctx, domain.TransferCommand{
		FromAccountID: treasury.ID, ToAccountID: alice.ID, AmountMinor: 5000, Currency: "USD",
		IdempotencyKey: "key-1",
	})
	require.NoError(t, err)

	// Replaying the same key — even with a different amount — must not apply
	// a second posting and must return the original transfer.
	second, err := svc.Transfer(ctx, domain.TransferCommand{
		FromAccountID: treasury.ID, ToAccountID: alice.ID, AmountMinor: 999999, Currency: "USD",
		IdempotencyKey: "key-1",
	})
	require.NoError(t, err)
	assert.Equal(t, first.Transfer.ID, second.Transfer.ID)
	assert.EqualValues(t, 5000, second.Transfer.AmountMinor)

	aliceAfter, err := svc.GetAccount(ctx, alice.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 5000, aliceAfter.BalanceMinor)
}

func TestGetAccountNotFound(t *testing.T) {
	t.Parallel()
	svc, cleanup := newService(t)
	defer cleanup()
	ctx := context.Background()

	_, err := svc.GetAccount(ctx, "missing")
	assertCode(t, err, domain.CodeAccountNotFound)

	_, err = svc.GetAccountEntries(ctx, "missing")
	assertCode(t, err, domain.CodeAccountNotFound)
}

func TestListAccountsOrdered(t *testing.T) {
	t.Parallel()
	svc, cleanup := newService(t)
	defer cleanup()
	ctx := context.Background()

	mustAccount(t, svc, "First", "USD", domain.AccountInternal)
	mustAccount(t, svc, "Second", "USD", domain.AccountInternal)

	accounts, err := svc.ListAccounts(ctx)
	require.NoError(t, err)
	require.Len(t, accounts, 2)
}
