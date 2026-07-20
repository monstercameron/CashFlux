// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strconv"
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

// titleCaseWords converts a string to title case (first letter of each word
// capitalised, rest lowercased). Used when normalising free-text institution
// names on save (MIA-extend #445-10).
func titleCaseWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

// fmtMoneyCompact renders a money value short enough to sit on a chart axis:
// "$250k", "$1.2M", "$840". Axis gridlines need magnitude at a glance, and a
// full "$250,000.00" on every tick crowds the plot it is supposed to explain.
// Display-only, and never used where an exact figure is the point — the labels
// beside the chart carry those in full.
func fmtMoneyCompact(m money.Money) string {
	sym := currency.Symbol(m.Currency)
	major := float64(m.Amount) / math.Pow(10, float64(currency.Decimals(m.Currency)))
	sign := ""
	if major < 0 {
		sign, major = "−", -major
	}
	switch {
	case major >= 1_000_000:
		return sign + sym + trimZero(major/1_000_000) + "M"
	case major >= 1_000:
		return sign + sym + trimZero(major/1_000) + "k"
	default:
		return sign + sym + strconv.FormatFloat(major, 'f', -1, 64)
	}
}

// trimZero formats a scaled figure with one decimal, dropping a trailing ".0"
// so an axis reads "$250k" rather than "$250.0k".
func trimZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}
