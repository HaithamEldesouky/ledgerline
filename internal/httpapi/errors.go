package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/haithamEldesouky/ledgerline/internal/domain"
)

// errorDetail is the body of an API error.
type errorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// errorBody is the envelope returned for every error response.
type errorBody struct {
	Error errorDetail `json:"error"`
}

// statusForCode maps a domain error code to an HTTP status code.
func statusForCode(code domain.ErrorCode) int {
	switch code {
	case domain.CodeAccountNotFound:
		return http.StatusNotFound
	case domain.CodeCurrencyMismatch:
		return http.StatusConflict
	case domain.CodeInsufficientFunds:
		return http.StatusUnprocessableEntity
	case domain.CodeInvalidAmount, domain.CodeSameAccountTransfer, domain.CodeValidation:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// writeError maps any error to a structured JSON response. Domain errors are
// translated to meaningful status codes; anything else becomes a 500 and is
// logged without leaking internal details to the client.
func (api *API) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var derr *domain.Error
	if errors.As(err, &derr) {
		writeJSON(w, statusForCode(derr.Code), errorBody{
			Error: errorDetail{
				Code:    string(derr.Code),
				Message: derr.Message,
				Details: derr.Details,
			},
		})
		return
	}

	api.log.ErrorContext(r.Context(), "unhandled error", slog.Any("err", err))
	writeJSON(w, http.StatusInternalServerError, errorBody{
		Error: errorDetail{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "an unexpected error occurred",
		},
	})
}
