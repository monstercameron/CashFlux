// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// fmtMoney renders a Money value for display in the candidate-C accounting
// figure style: thousands grouping, the currency symbol, and parentheses for
// negatives — "$1,234.56" for positives and "($240.55)" for negatives. This is
// the single money-display formatter so figures read identically on every
// screen (C2). It is display-only; editable inputs format with
// money.FormatMinor (no symbol, plain minus) so a value never round-trips
// through parentheses.
func fmtMoney(m money.Money) string {
	return money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
}

// figTone returns the candidate-C figure color class for a signed value:
// up (green) for positive, down (red) for negative, empty for zero.
func figTone(m money.Money) string {
	switch {
	case m.IsNegative():
		return "text-down"
	case m.Amount > 0:
		return "text-up"
	default:
		return ""
	}
}

// amountClass picks the green/red amount class for a money value.
func amountClass(m money.Money) string {
	if m.IsNegative() {
		return "amount amount-expense"
	}
	return "amount amount-income"
}

// accentFor returns the "pos"/"neg" stat accent for a money value.
func accentFor(m money.Money) string {
	if m.IsNegative() {
		return "neg"
	}
	return "pos"
}

// ownerVisibleTo reports whether an entity with the given ownerID should be
// shown when activeMemberID is the current member view. When activeMemberID is
// "" (everyone view) all entities are visible. Otherwise, group-/household-owned
// entities (GroupOwnerID) are always shown, and individual entities are shown
// only when their OwnerID matches the active member. Consistent with the
// dashboard's scoping convention (L21 / C278).
func ownerVisibleTo(ownerID, activeMemberID string) bool {
	if activeMemberID == "" {
		return true
	}
	return ownerID == activeMemberID || ownerID == domain.GroupOwnerID
}

// humanizeType turns an enum like "credit_card" into "Credit card".
func humanizeType(t string) string {
	t = strings.ReplaceAll(t, "_", " ")
	if t == "" {
		return t
	}
	return strings.ToUpper(t[:1]) + t[1:]
}
