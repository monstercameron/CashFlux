// SPDX-License-Identifier: MIT

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// fmtLedgerAmount renders a transaction amount for the ledger. Income is green
// and explicitly signed; every other amount is an unsigned magnitude in neutral ink.
func fmtLedgerAmount(m money.Money, isTransfer bool) (text, tone string) {
	decimals := currency.Decimals(m.Currency)
	symbol := currency.Symbol(m.Currency)
	amount := ledgerMagnitude(m.Amount, decimals)
	if isTransfer {
		return symbol + amount, ""
	}

	if m.Amount > 0 {
		return "+" + symbol + amount, "text-up"
	}
	if m.Amount == 0 {
		return symbol + amount, ""
	}
	return symbol + amount, "txn-amount-expense"
}

// fmtSignedMoney renders a signed amount with its currency symbol. Positive and
// negative values carry an explicit sign; zero keeps the currency symbol without
// a sign so it cannot appear as an unlabelled magnitude.
func fmtTxnSignedMoney(m money.Money) string {
	symbol := currency.Symbol(m.Currency)
	amount := ledgerMagnitude(m.Amount, currency.Decimals(m.Currency))
	switch {
	case m.Amount < 0:
		return "−" + symbol + amount
	case m.Amount > 0:
		return "+" + symbol + amount
	default:
		return symbol + amount
	}
}

// ledgerMagnitude formats an absolute minor-unit magnitude without overflowing
// for the minimum int64 value.
func ledgerMagnitude(amount int64, decimals int) string {
	var magnitude uint64
	if amount < 0 {
		magnitude = uint64(-(amount + 1)) + 1
	} else {
		magnitude = uint64(amount)
	}
	if decimals <= 0 {
		return money.Group(strconv.FormatUint(magnitude, 10))
	}
	divisor := uint64(1)
	for range decimals {
		divisor *= 10
	}
	whole := strconv.FormatUint(magnitude/divisor, 10)
	frac := strconv.FormatUint(magnitude%divisor, 10)
	if len(frac) < decimals {
		frac = strings.Repeat("0", decimals-len(frac)) + frac
	}
	return money.Group(whole + "." + frac)
}
