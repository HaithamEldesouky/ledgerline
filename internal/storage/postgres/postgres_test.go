package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
	"github.com/haithamEldesouky/ledgerline/internal/storage/postgres"
)

// setupRepo connects to the database named by DATABASE_URL, applies the schema
// migration and returns a ready repository. The whole test is skipped when no
// database is configured, so local runs (and CI jobs without Postgres) stay
// green while the integration job exercises the real adapter.
func setupRepo(t *testing.T) *postgres.Repository {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	ctx := context.Background()

	// Apply the migration. With no query arguments, pgx uses the simple
	// protocol, which permits the multi-statement migration script.
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", "0001_init.sql"))
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(schema))
	require.NoError(t, err)
	pool.Close()

	repo, err := postgres.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestPostgresTransferFlow(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.HealthCheck(ctx))

	treasury, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Treasury", Currency: "USD", Type: domain.AccountExternal,
	})
	require.NoError(t, err)
	alice, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Alice", Currency: "USD", Type: domain.AccountInternal,
	})
	require.NoError(t, err)

	result, err := repo.ExecuteTransfer(ctx, domain.ExecuteTransferInput{
		FromAccountID: treasury.ID, ToAccountID: alice.ID, AmountMinor: 10000, Currency: "USD",
	})
	require.NoError(t, err)
	require.Len(t, result.Entries, 2)

	got, found, err := repo.GetAccount(ctx, alice.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.EqualValues(t, 10000, got.BalanceMinor)

	entries, err := repo.ListEntriesByAccount(ctx, alice.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.DirectionCredit, entries[0].Direction)
}

func TestPostgresInsufficientFunds(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	alice, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Alice", Currency: "USD", Type: domain.AccountInternal,
	})
	require.NoError(t, err)
	sink, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Sink", Currency: "USD", Type: domain.AccountExternal,
	})
	require.NoError(t, err)

	_, err = repo.ExecuteTransfer(ctx, domain.ExecuteTransferInput{
		FromAccountID: alice.ID, ToAccountID: sink.ID, AmountMinor: 100, Currency: "USD",
	})
	var derr *domain.Error
	require.ErrorAs(t, err, &derr)
	assert.Equal(t, domain.CodeInsufficientFunds, derr.Code)
}

func TestPostgresIdempotency(t *testing.T) {
	repo := setupRepo(t)
	ctx := context.Background()

	treasury, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Treasury", Currency: "USD", Type: domain.AccountExternal,
	})
	require.NoError(t, err)
	alice, err := repo.CreateAccount(ctx, domain.CreateAccountInput{
		Name: "Alice", Currency: "USD", Type: domain.AccountInternal,
	})
	require.NoError(t, err)

	key := "pg-idem-" + alice.ID
	first, err := repo.ExecuteTransfer(ctx, domain.ExecuteTransferInput{
		FromAccountID: treasury.ID, ToAccountID: alice.ID, AmountMinor: 5000, Currency: "USD", IdempotencyKey: key,
	})
	require.NoError(t, err)

	second, err := repo.ExecuteTransfer(ctx, domain.ExecuteTransferInput{
		FromAccountID: treasury.ID, ToAccountID: alice.ID, AmountMinor: 5000, Currency: "USD", IdempotencyKey: key,
	})
	require.NoError(t, err)
	assert.Equal(t, first.Transfer.ID, second.Transfer.ID)

	got, _, err := repo.GetAccount(ctx, alice.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 5000, got.BalanceMinor)
}
