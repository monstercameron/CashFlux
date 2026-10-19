// SPDX-License-Identifier: MIT

package domain

import "github.com/monstercameron/CashFlux/internal/money"

// WithBaseCurrency resolves legacy implicit budget currencies without changing
// any explicit currency or mutating the receiver. An unset household uses USD,
// the same default as a new workspace.
func (b Budget) WithBaseCurrency(base string) Budget {
	if base == "" {
		base = "USD"
	}
	if b.Limit.Currency == "" {
		b.Limit = money.New(b.Limit.Amount, base)
	}
	if b.TargetAmount.Currency == "" && b.HasTarget() {
		b.TargetAmount = money.New(b.TargetAmount.Amount, b.Limit.Currency)
	}
	return b
}
