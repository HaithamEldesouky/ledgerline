package domain

import "time"

// AccountType classifies an account.
//
//   - internal: a normal customer/wallet account whose balance may never go
//     negative — debits are rejected when funds are insufficient.
//   - external: a funding source/sink (treasury, card-network gateway, bank
//     settlement account). It may carry a negative balance, modelling money
//     entering or leaving the system while preserving the double-entry
//     invariant that every debit has a matching credit.
type AccountType string

const (
	AccountInternal AccountType = "internal"
	AccountExternal AccountType = "external"
)

// Account is a ledger account with a balance denominated in minor units.
type Account struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Currency     string      `json:"currency"`
	Type         AccountType `json:"type"`
	BalanceMinor int64       `json:"balanceMinor"`
	CreatedAt    time.Time   `json:"createdAt"`
}

// EntryDirection is the side of a double-entry posting.
type EntryDirection string

const (
	DirectionDebit  EntryDirection = "debit"
	DirectionCredit EntryDirection = "credit"
)

// LedgerEntry is one immutable side of a posting. Every transfer produces
// exactly two entries — one debit and one credit — of equal amount.
type LedgerEntry struct {
	ID                string         `json:"id"`
	TransferID        string         `json:"transferId"`
	AccountID         string         `json:"accountId"`
	Direction         EntryDirection `json:"direction"`
	AmountMinor       int64          `json:"amountMinor"`
	BalanceAfterMinor int64          `json:"balanceAfterMinor"`
	CreatedAt         time.Time      `json:"createdAt"`
}

// Transfer records an atomic movement of money between two accounts.
type Transfer struct {
	ID             string    `json:"id"`
	FromAccountID  string    `json:"fromAccountId"`
	ToAccountID    string    `json:"toAccountId"`
	AmountMinor    int64     `json:"amountMinor"`
	Currency       string    `json:"currency"`
	IdempotencyKey string    `json:"idempotencyKey,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// TransferResult is a posted transfer together with its two ledger entries.
type TransferResult struct {
	Transfer Transfer      `json:"transfer"`
	Entries  []LedgerEntry `json:"entries"`
}
