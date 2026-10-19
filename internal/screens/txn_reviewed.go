// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// markTxnReviewedFromKeyboard confirms the focused ledger transaction. The
// write uses the same appstate persistence path as the row and review controls.
func markTxnReviewedFromKeyboard(txnID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	txn, ok := app.TransactionByID(txnID)
	if !ok || txn.Reviewed {
		return
	}
	var changed bool
	txn, changed = txnReviewed(txn)
	if !changed {
		return
	}
	if err := app.PutTransaction(txn); err != nil {
		uistate.PostNotice(err.Error(), true)
		return
	}
	uistate.BumpDataRevision()
	uistate.PostNotice(uistate.T("txnkeys.reviewedToast"), false)
}

func txnReviewed(txn domain.Transaction) (domain.Transaction, bool) {
	if txn.Reviewed {
		return txn, false
	}
	txn.Reviewed = true
	return txn, true
}
