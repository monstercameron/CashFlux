// SPDX-License-Identifier: MIT

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
)

// fmtLedgerAmount renders a transaction amount using the shared ledger convention:
// income is green and signed, expenses are red and signed, and transfers are neutral.
func fmtLedgerAmount(m money.Money, isTransfer bool) (text, tone string) {
	return uiw.FormatMoneyFig(m, uiw.MoneyLedger, isTransfer)
}

// fmtSignedMoney renders a signed amount with its currency symbol. Positive and
// negative values carry an explicit sign; zero keeps the currency symbol without
// a sign so it cannot appear as an unlabelled magnitude.
func fmtTxnSignedMoney(m money.Money) string {
	text, _ := uiw.FormatMoneyFig(m, uiw.MoneySigned, false)
	return text
}
