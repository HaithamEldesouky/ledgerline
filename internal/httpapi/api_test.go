package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
	"github.com/haithamEldesouky/ledgerline/internal/httpapi"
	"github.com/haithamEldesouky/ledgerline/internal/observability"
	"github.com/haithamEldesouky/ledgerline/internal/storage/memory"
)

func buildHandler(t *testing.T, rps float64, burst int) http.Handler {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	limiter := httpapi.NewRateLimiter(rps, burst)
	t.Cleanup(limiter.Close)
	return httpapi.NewRouter(httpapi.RouterDeps{
		Service:     domain.NewService(repo),
		Repository:  repo,
		Logger:      observability.NewLogger("test", "error"),
		Metrics:     httpapi.NewMetrics(),
		RateLimiter: limiter,
	})
}

func do(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func createAccount(t *testing.T, h http.Handler, name, currency, typ string) domain.Account {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/api/v1/accounts",
		map[string]any{"name": name, "currency": currency, "type": typ}, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var acc domain.Account
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &acc))
	return acc
}

func TestEndToEndTransferFlow(t *testing.T) {
	t.Parallel()
	h := buildHandler(t, 1000, 1000)

	treasury := createAccount(t, h, "Treasury", "USD", "external")
	alice := createAccount(t, h, "Alice", "USD", "internal")

	// Fund Alice from the treasury.
	rec := do(t, h, http.MethodPost, "/api/v1/transfers", map[string]any{
		"fromAccountId": treasury.ID,
		"toAccountId":   alice.ID,
		"amountMinor":   10000,
		"currency":      "USD",
	}, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var result domain.TransferResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result.Entries, 2)

	// Balance reflects the credit.
	rec = do(t, h, http.MethodGet, "/api/v1/accounts/"+alice.ID, nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var got domain.Account
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.EqualValues(t, 10000, got.BalanceMinor)

	// Ledger history has one entry for Alice.
	rec = do(t, h, http.MethodGet, "/api/v1/accounts/"+alice.ID+"/transactions", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var entries []domain.LedgerEntry
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &entries))
	require.Len(t, entries, 1)

	// Listing returns both accounts.
	rec = do(t, h, http.MethodGet, "/api/v1/accounts", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var list []domain.Account
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	assert.Len(t, list, 2)
}

func TestErrorResponses(t *testing.T) {
	t.Parallel()
	h := buildHandler(t, 1000, 1000)

	t.Run("malformed body is 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader("{not json"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unknown field is 400", func(t *testing.T) {
		rec := do(t, h, http.MethodPost, "/api/v1/accounts",
			map[string]any{"name": "x", "currency": "USD", "bogus": 1}, nil)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing account is 404", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/api/v1/accounts/00000000-0000-0000-0000-000000000000", nil, nil)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		var body map[string]map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "ACCOUNT_NOT_FOUND", body["error"]["code"])
	})

	t.Run("insufficient funds is 422", func(t *testing.T) {
		alice := createAccount(t, h, "Alice2", "USD", "internal")
		sink := createAccount(t, h, "Sink", "USD", "external")
		rec := do(t, h, http.MethodPost, "/api/v1/transfers", map[string]any{
			"fromAccountId": alice.ID,
			"toAccountId":   sink.ID,
			"amountMinor":   100,
			"currency":      "USD",
		}, nil)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	})
}

func TestIdempotentTransferViaHeader(t *testing.T) {
	t.Parallel()
	h := buildHandler(t, 1000, 1000)

	treasury := createAccount(t, h, "Treasury", "USD", "external")
	alice := createAccount(t, h, "Alice", "USD", "internal")

	headers := map[string]string{"Idempotency-Key": "abc-123"}
	body := map[string]any{
		"fromAccountId": treasury.ID,
		"toAccountId":   alice.ID,
		"amountMinor":   5000,
		"currency":      "USD",
	}

	rec1 := do(t, h, http.MethodPost, "/api/v1/transfers", body, headers)
	require.Equal(t, http.StatusCreated, rec1.Code)
	var first domain.TransferResult
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &first))

	rec2 := do(t, h, http.MethodPost, "/api/v1/transfers", body, headers)
	require.Equal(t, http.StatusCreated, rec2.Code)
	var second domain.TransferResult
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &second))

	assert.Equal(t, first.Transfer.ID, second.Transfer.ID)

	// Applied exactly once.
	rec := do(t, h, http.MethodGet, "/api/v1/accounts/"+alice.ID, nil, nil)
	var got domain.Account
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.EqualValues(t, 5000, got.BalanceMinor)
}

func TestRateLimiting(t *testing.T) {
	t.Parallel()
	h := buildHandler(t, 1, 1) // 1 request burst

	rec1 := do(t, h, http.MethodGet, "/api/v1/accounts", nil, nil)
	assert.Equal(t, http.StatusOK, rec1.Code)

	rec2 := do(t, h, http.MethodGet, "/api/v1/accounts", nil, nil)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)
}

func TestObservabilityEndpoints(t *testing.T) {
	t.Parallel()
	h := buildHandler(t, 1000, 1000)

	rec := do(t, h, http.MethodGet, "/health/live", nil, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ok")

	rec = do(t, h, http.MethodGet, "/health/ready", nil, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "ready")

	// Generate a request, then confirm metrics are exported.
	_ = do(t, h, http.MethodGet, "/api/v1/accounts", nil, nil)
	rec = do(t, h, http.MethodGet, "/metrics", nil, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "http_requests_total")
}
