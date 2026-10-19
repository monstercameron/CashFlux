// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnreview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"reflect"
)

// markTxnReviewedFromKeyboard confirms the focused ledger transaction. The
// write uses the same appstate persistence path as the row and review controls.
func markTxnReviewedFromKeyboard(txnID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	txn, ok := app.TransactionByID(txnID)
	if !ok {
		return
	}
	var changed bool
	txn, changed = txnReviewed(txn)
	if !changed {
		return
	}
	_ = commitUndoable(uistate.T("txnkeys.reviewedToast"), []string{txnID}, func() error {
		return app.PutTransactionsAtomic([]domain.Transaction{txn})
	})
}

func txnReviewed(txn domain.Transaction) (domain.Transaction, bool) {
	confirmed := txnreview.Confirm(txn, txn.CategoryID)
	return confirmed, !reflect.DeepEqual(txn, confirmed)
}
