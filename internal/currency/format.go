// SPDX-License-Identifier: MIT

package currency

import "github.com/monstercameron/CashFlux/internal/money"

// FormatAccounting converts m into the target currency through the rate table
// and renders it accounting-style for display — the target currency's symbol and
// minor-unit digits, with negatives in parentheses (via money.FormatAccounting).
// It returns an error when the conversion can't be made (missing or non-positive
// rate). Formatting for display lives here, at the currency edge, so the money
// package stays free of the currency registry.
func (r Rates) FormatAccounting(m money.Money, toCurrency string) (string, error) {
	conv, err := r.Convert(m, toCurrency)
	if err != nil {
		return "", err
	}
	return money.FormatAccounting(conv.Amount, Decimals(toCurrency), Symbol(toCurrency)), nil
}

// FormatInBase converts m into the rate table's base currency and renders it
// accounting-style — the common case for multi-currency aggregation display.
func (r Rates) FormatInBase(m money.Money) (string, error) {
	return r.FormatAccounting(m, r.Base)
}
