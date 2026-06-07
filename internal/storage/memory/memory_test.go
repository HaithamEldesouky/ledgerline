package memory_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
	"github.com/haithamEldesouky/ledgerline/internal/storage/memory"
)

func TestConcurrentIdempotentTransfers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	treasury, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Treasury", Currency: "USD", Type: domain.AccountExternal,
	})
	require.NoError(t, err)
	alice, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Alice", Currency: "USD", Type: domain.AccountInternal,
	})
	require.NoError(t, err)

	const goroutines = 50
	var wg sync.WaitGroup
	results := make([]domain.TransferResult, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = repo.ExecuteTransfer(ctx, domain.ExecuteTransferInput{
				FromAccountID:  treasury.ID,
				ToAccountID:    alice.ID,
				AmountMinor:    100,
				Currency:       "USD",
				IdempotencyKey: "same-key",
			})
		}(i)
	}
	wg.Wait()

	// Every call must succeed and resolve to the exact same transfer.
	transferID := ""
	for i := 0; i < goroutines; i++ {
		require.NoError(t, errs[i])
		if transferID == "" {
			transferID = results[i].Transfer.ID
		}
		assert.Equal(t, transferID, results[i].Transfer.ID)
	}

	// The posting must have been applied exactly once.
	got, found, err := repo.GetAccount(ctx, alice.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 100, got.BalanceMinor)
}

func TestListEntriesByAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	treasury, _ := repo.CreateAccount(ctx, domain.CreateAccountInput{Name: "T", Currency: "USD", Type: domain.AccountExternal})
	alice, _ := repo.CreateAccount(ctx, domain.CreateAccountInput{Name: "A", Currency: "USD", Type: domain.AccountInternal})

	_, err := repo.ExecuteTransfer(ctx, domain.ExecuteTransferInput{
		FromAccountID: treasury.ID, ToAccountID: alice.ID, AmountMinor: 250, Currency: "USD",
	})
	require.NoError(t, err)

	entries, err := repo.ListEntriesByAccount(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.DirectionCredit, entries[0].Direction)
	assert.EqualValues(t, 250, entries[0].BalanceAfterMinor)
}
