// SPDX-License-Identifier: MIT

package domain

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// SharedExpenseShare is one member's portion of a shared expense.
type SharedExpenseShare struct {
	MemberID string      `json:"memberId"`
	Amount   money.Money `json:"amount"`
}

// SharedExpense is a cost fronted by one member and split among the household —
// the persisted record behind the Split screen's "settle up" ledger. The shares
// are expected to sum to the amount the payer fronted.
type SharedExpense struct {
	ID      string               `json:"id"`
	Desc    string               `json:"desc,omitempty"`
	Date    time.Time            `json:"date,omitempty"`
	PayerID string               `json:"payerId"`
	Shares  []SharedExpenseShare `json:"shares"`
	Custom  map[string]any       `json:"custom,omitempty"`
}

// Total returns the sum of the expense's shares (the amount the payer fronted),
// in the currency of the first share; a share-less expense totals zero with an
// empty currency.
func (e SharedExpense) Total() money.Money {
	if len(e.Shares) == 0 {
		return money.Money{}
	}
	total := money.Zero(e.Shares[0].Amount.Currency)
	for _, s := range e.Shares {
		total.Amount += s.Amount.Amount
	}
	return total
}

// Settlement records a payment from one member to another that squares up shared
// expenses — the reverse side of the settle-up ledger.
type Settlement struct {
	ID     string      `json:"id"`
	FromID string      `json:"fromId"`
	ToID   string      `json:"toId"`
	Amount money.Money `json:"amount"`
	Date   time.Time   `json:"date,omitempty"`
}
