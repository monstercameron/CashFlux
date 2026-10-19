// SPDX-License-Identifier: MIT

package domain

import "time"

// DebtPaymentSource identifies how a debt-payment link was established.
type DebtPaymentSource string

const (
	// DebtPaymentConfirmed records a user-confirmed payment link.
	DebtPaymentConfirmed DebtPaymentSource = "confirmed"
	// DebtPaymentRule records a link applied from a confirmed payee rule.
	DebtPaymentRule DebtPaymentSource = "rule"
)

// DebtPaymentMethod explains how the payment allocation was calculated.
type DebtPaymentMethod string

const (
	// DebtPaymentStatement means the amounts came from a supplied statement split.
	DebtPaymentStatement DebtPaymentMethod = "statement"
	// DebtPaymentCard means a card payment without a supplied split was assigned to principal.
	DebtPaymentCard DebtPaymentMethod = "card_payment"
	// DebtPaymentAmortized means the allocation uses an explicitly marked estimate.
	DebtPaymentAmortized DebtPaymentMethod = "amortization_estimate"
)

// DebtPaymentLink records a confirmed relationship between transaction rows and
// the debt account they paid, including the allocation of cash paid.
type DebtPaymentLink struct {
	ID              string            `json:"id"`
	TransactionIDs  []string          `json:"transactionIds"`
	DebtAccountID   string            `json:"debtAccountId"`
	Currency        string            `json:"currency"`
	PrincipalMinor  int64             `json:"principalMinor"`
	InterestMinor   int64             `json:"interestMinor"`
	FeesMinor       int64             `json:"feesMinor"`
	EscrowMinor     int64             `json:"escrowMinor"`
	Estimated       bool              `json:"estimated"`
	Method          DebtPaymentMethod `json:"method"`
	Source          DebtPaymentSource `json:"source"`
	CreatedRevision uint64            `json:"createdRevision"`
	CreatedAt       time.Time         `json:"createdAt,omitempty"`
}

// DebtPayeeRule remembers a confirmed payee-to-debt association within one
// workspace. SourceAccountID may be empty to match transactions from any account.
type DebtPayeeRule struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspaceId"`
	Payee           string    `json:"payee"`
	DebtAccountID   string    `json:"debtAccountId"`
	SourceAccountID string    `json:"sourceAccountId,omitempty"`
	CreatedAt       time.Time `json:"createdAt,omitempty"`
}
