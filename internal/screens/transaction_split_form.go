// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// TransactionSplitFormProps configures the modal-hosted split editor: the id of
// the transaction to split, and OnDone, called after a successful save (or when
// the transaction has vanished) so the host can close the modal.
type TransactionSplitFormProps struct {
	TxnID  string
	OnDone func()
}

// TransactionSplitForm hosts the SplitEditor inside the shell-root flip modal
// (TxnSplitHost), so the split-into-categories flow is reachable from the live
// widgetized transactions view — the editor itself previously only rendered in
// the classic table's inline-edit row. It looks the transaction up live (an edit
// always works against current data), delegates all draft state, remainder math,
// and Σ=amount validation to SplitEditor, and on save persists via PutTransaction
// with the same toast/bump behavior as the classic view's saveSplits.
func TransactionSplitForm(props TransactionSplitFormProps) ui.Node {
	return ui.CreateElement(transactionSplitForm, props)
}

func transactionSplitForm(props TransactionSplitFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	// Look the transaction up live so the editor always seeds from current data.
	var txn domain.Transaction
	found := false
	for _, t := range app.Transactions() {
		if t.ID == props.TxnID {
			txn, found = t, true
			break
		}
	}
	if !found {
		// Deleted out from under the modal (e.g. by another member/agent) — nothing
		// to split; close.
		if props.OnDone != nil {
			props.OnDone()
		}
		return Fragment()
	}

	save := func(updated domain.Transaction) {
		if err := app.PutTransaction(updated); err != nil {
			uistate.PostNotice(err.Error(), false)
			return
		}
		uistate.BumpDataRevision()
		if updated.HasSplits() {
			uistate.PostNotice(uistate.T("splitEditor.saved"), false)
		} else {
			uistate.PostNotice(uistate.T("splitEditor.cleared"), false)
		}
		if props.OnDone != nil {
			props.OnDone()
		}
	}

	return ui.CreateElement(SplitEditor, splitEditorProps{
		Txn:        txn,
		Categories: app.Categories(),
		OnSave:     save,
	})
}
