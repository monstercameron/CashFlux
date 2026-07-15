// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// signedMoney renders a money value with an explicit leading sign and currency
// symbol — "−$1,400.00" for outflows, "+$1,200.00" for inflows — the form the
// projected-balance driver list (AC13) uses so a cash flow's direction reads at a
// glance. Distinct from fmtMoney, which parenthesizes negatives for accounting
// figures; drivers read better as signed deltas.
func signedMoney(m money.Money, dec int) string {
	sym := currency.Symbol(m.Currency)
	amt := m.Amount
	sign := "+"
	if amt < 0 {
		sign = "−" // minus sign (matches the app's figure typography)
		amt = -amt
	}
	return sign + sym + money.FormatMinor(amt, dec)
}

// fmtShortDate renders a date as a compact "Jan 2" label for the projected-balance
// line and its driver list (AC13).
func fmtShortDate(t time.Time) string {
	return t.Format("Jan 2")
}
