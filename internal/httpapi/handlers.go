package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
)

// API holds the dependencies shared by all HTTP handlers.
type API struct {
	svc  *domain.Service
	repo domain.Repository
	log  *slog.Logger
}

type createAccountRequest struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Type     string `json:"type"`
}

type createTransferRequest struct {
	FromAccountID string `json:"fromAccountId"`
	ToAccountID   string `json:"toAccountId"`
	AmountMinor   int64  `json:"amountMinor"`
	Currency      string `json:"currency"`
}

func (api *API) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		api.writeError(w, r, domain.ErrValidation("invalid request body: "+err.Error()))
		return
	}

	account, err := api.svc.CreateAccount(r.Context(), domain.CreateAccountCommand{
		Name:     req.Name,
		Currency: req.Currency,
		Type:     domain.AccountType(req.Type),
	})
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (api *API) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := api.svc.ListAccounts(r.Context())
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (api *API) getAccount(w http.ResponseWriter, r *http.Request) {
	account, err := api.svc.GetAccount(r.Context(), r.PathValue("id"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (api *API) listAccountEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := api.svc.GetAccountEntries(r.Context(), r.PathValue("id"))
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (api *API) createTransfer(w http.ResponseWriter, r *http.Request) {
	var req createTransferRequest
	if err := decodeJSON(w, r, &req); err != nil {
		api.writeError(w, r, domain.ErrValidation("invalid request body: "+err.Error()))
		return
	}

	result, err := api.svc.Transfer(r.Context(), domain.TransferCommand{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		AmountMinor:    req.AmountMinor,
		Currency:       req.Currency,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		api.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (api *API) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) ready(w http.ResponseWriter, r *http.Request) {
	if err := api.repo.HealthCheck(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"reason": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
