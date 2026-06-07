package domain

import "fmt"

// ErrorCode is a transport-agnostic, machine-readable classification of a
// domain failure. The HTTP layer maps each code onto an appropriate status
// code (see internal/httpapi/errors.go).
type ErrorCode string

const (
	CodeAccountNotFound     ErrorCode = "ACCOUNT_NOT_FOUND"
	CodeCurrencyMismatch    ErrorCode = "CURRENCY_MISMATCH"
	CodeInsufficientFunds   ErrorCode = "INSUFFICIENT_FUNDS"
	CodeInvalidAmount       ErrorCode = "INVALID_AMOUNT"
	CodeSameAccountTransfer ErrorCode = "SAME_ACCOUNT_TRANSFER"
	CodeValidation          ErrorCode = "VALIDATION_ERROR"
)

// Error is the single error type returned by the domain. It carries a stable
// code, a human-readable message and optional structured details.
type Error struct {
	Code    ErrorCode
	Message string
	Details map[string]any
}

func (e *Error) Error() string { return e.Message }

func newError(code ErrorCode, message string, details map[string]any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}

// ErrAccountNotFound indicates the referenced account does not exist.
func ErrAccountNotFound(accountID string) *Error {
	return newError(
		CodeAccountNotFound,
		fmt.Sprintf("account %q was not found", accountID),
		map[string]any{"accountId": accountID},
	)
}

// ErrCurrencyMismatch indicates a transfer currency differs from an account's.
func ErrCurrencyMismatch(expected, actual string) *Error {
	return newError(
		CodeCurrencyMismatch,
		fmt.Sprintf("currency mismatch: expected %s, received %s", expected, actual),
		map[string]any{"expected": expected, "actual": actual},
	)
}

// ErrInsufficientFunds indicates an internal account cannot be debited.
func ErrInsufficientFunds(accountID string, balanceMinor, amountMinor int64) *Error {
	return newError(
		CodeInsufficientFunds,
		fmt.Sprintf("account %q has insufficient funds for this transfer", accountID),
		map[string]any{
			"accountId":    accountID,
			"balanceMinor": balanceMinor,
			"amountMinor":  amountMinor,
		},
	)
}

// ErrInvalidAmount indicates a non-positive transfer amount.
func ErrInvalidAmount(amountMinor int64) *Error {
	return newError(
		CodeInvalidAmount,
		fmt.Sprintf("transfer amount must be a positive integer in minor units, got %d", amountMinor),
		map[string]any{"amountMinor": amountMinor},
	)
}

// ErrSameAccountTransfer indicates the source and destination are identical.
func ErrSameAccountTransfer(accountID string) *Error {
	return newError(
		CodeSameAccountTransfer,
		"the source and destination accounts must be different",
		map[string]any{"accountId": accountID},
	)
}

// ErrValidation indicates malformed or missing input.
func ErrValidation(message string) *Error {
	return newError(CodeValidation, message, nil)
}
