//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// fmtMoney renders a Money value for display, e.g. "$1234.56" or "-$240.55".
func fmtMoney(m money.Money) string {
	sym := currency.Symbol(m.Currency)
	s := money.FormatMinor(m.Amount, currency.Decimals(m.Currency))
	if strings.HasPrefix(s, "-") {
		return "-" + sym + s[1:]
	}
	return sym + s
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

// humanizeType turns an enum like "credit_card" into "Credit card".
func humanizeType(t string) string {
	t = strings.ReplaceAll(t, "_", " ")
	if t == "" {
		return t
	}
	return strings.ToUpper(t[:1]) + t[1:]
}
