// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// receiptCountLabel is the plain-English label for a transaction's attached
// receipts, e.g. "1 receipt attached" / "3 receipts attached" (L29).
func receiptCountLabel(n int) string {
	if n == 1 {
		return uistate.T("transactions.receiptAttached", n)
	}
	return uistate.T("transactions.receiptsAttached", n)
}

func signedEditAmount(raw, direction, currencyCode string) int64 {
	amount, err := money.ParseMinor(strings.TrimSpace(raw), currency.Decimals(currencyCode))
	if err != nil {
		return 0
	}
	if amount < 0 {
		amount = -amount
	}
	if direction == "income" {
		return amount
	}
	return -amount
}
