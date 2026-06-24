// SPDX-License-Identifier: MIT

package domain

import "github.com/monstercameron/CashFlux/internal/money"

// CategorySplit assigns part of a transaction's amount to a category. A
// transaction with one bank charge but several categorized lines (a grocery
// receipt: produce, dairy, household) carries its breakdown as Splits, so it
// counts once against the account yet reports per-category spend.
type CategorySplit struct {
	CategoryID string      `json:"categoryId"`
	Amount     money.Money `json:"amount"`
}

// SplitsTotal returns the sum of the splits, in the currency of the first split
// (an empty list totals zero with an empty currency). Splits are assumed to share
// a currency; mixed currencies are summed by minor amount only.
func SplitsTotal(splits []CategorySplit) money.Money {
	if len(splits) == 0 {
		return money.Money{}
	}
	total := money.Zero(splits[0].Amount.Currency)
	for _, s := range splits {
		total.Amount += s.Amount.Amount
	}
	return total
}

// SplitsReconcile reports whether the splits sum exactly to amount (to the minor
// unit). A transaction with no splits reconciles trivially — it simply isn't
// split. The comparison is on the integer minor amount, so an unset split
// currency still reconciles against amount when the totals match.
func SplitsReconcile(amount money.Money, splits []CategorySplit) bool {
	if len(splits) == 0 {
		return true
	}
	return SplitsTotal(splits).Amount == amount.Amount
}

// HasSplits reports whether the transaction carries a category breakdown.
func (t Transaction) HasSplits() bool { return len(t.Splits) > 0 }

// SplitsReconcile reports whether this transaction's splits sum to its amount.
func (t Transaction) SplitsReconcile() bool { return SplitsReconcile(t.Amount, t.Splits) }
