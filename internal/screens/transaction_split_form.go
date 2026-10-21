// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
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
	var txn domain.Transaction
	found := false
	if app != nil {
		txn, found = app.TransactionByID(props.TxnID)
	}

	// Consume a receipt-derived proposal once so the editor can review its lines.
	proposedNote := ui.UseState("")
	saving := ui.UseState(false)
	saveLease := useTxnFormLease(props.TxnID)
	ui.UseEffect(func() func() {
		saving.Set(false)
		return nil
	}, props.TxnID)
	seededTxn := ui.UseState(domain.Transaction{})
	haveSeed := ui.UseState(false)
	draftGeneration := ui.UseState(0)
	ui.UseEffect(func() func() {
		if app != nil && !found {
			if props.OnDone != nil {
				props.OnDone()
			}
			return nil
		}
		if app == nil || !found {
			return nil
		}
		if splits, note, ok := uistate.TakeTxnSplitProposal(props.TxnID); ok {
			t := txn
			t.Splits = splits
			seededTxn.Set(t)
			proposedNote.Set(note)
			haveSeed.Set(true)
			draftGeneration.Set(draftGeneration.Get() + 1)
		}
		return nil
	}, props.TxnID, found)
	suggestionUse := ui.UseCallback(func(lines []domain.CategorySplit) {
		if len(lines) == 0 {
			return
		}
		live := txn
		live.Splits = lines
		seededTxn.Set(live)
		proposedNote.Set("")
		haveSeed.Set(true)
		draftGeneration.Set(draftGeneration.Get() + 1)
	}, app, props.TxnID, txn.Amount, txn.Payee, txn.Desc)
	revision := uint64(0)
	if app != nil {
		revision = app.Rev()
	}
	suggestionNode := ui.UseMemo(func() ui.Node {
		return ui.CreateElement(splitSuggestHint, splitSuggestProps{Txn: txn, OnUse: suggestionUse})
	}, app, revision, props.TxnID, txn.Amount, txn.Payee, txn.Desc, draftGeneration.Get())
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	if !found {
		return Fragment()
	}

	editorTxn := txn
	if haveSeed.Get() {
		// Keep the live transaction's mutable fields but carry the proposed splits.
		editorTxn.Splits = seededTxn.Get().Splits
	}

	save := func(updated domain.Transaction) {
		if saving.Get() {
			return
		}
		live, ok := app.TransactionByID(props.TxnID)
		if !ok {
			return
		}
		live.Splits = append([]domain.CategorySplit(nil), updated.Splits...)
		isCurrent := saveLease.check()
		saving.Set(true)
		label := uistate.T("splitEditor.cleared")
		if live.HasSplits() {
			label = uistate.T("splitEditor.r4Saved")
		}
		_ = CommitUndoable(app, label, []string{live.ID}, func() error { return app.PutTransactionsAtomic([]domain.Transaction{live}) }, func(err error) {
			if !isCurrent() {
				return
			}
			saving.Set(false)
			if err != nil {
				uistate.PostNotice(uistate.T("txint.splitNotSaved"), true)
				return
			}
			if props.OnDone != nil {
				props.OnDone()
			}
		})
	}

	return Fragment(
		// Keep suggestion handlers in their own component.
		suggestionNode,
		If(proposedNote.Get() != "",
			Div(css.Class("callout"), Attr("role", "status"), Attr("data-testid", "receipt-split-note"),
				Style(map[string]string{"margin-bottom": "0.5rem", "padding": "0.6rem 0.75rem",
					"border": "1px solid var(--border)", "border-radius": "8px", "background": "var(--bg-elev)"}),
				P(css.Class("t-body"), proposedNote.Get()))),
		ui.CreateElement(SplitEditor, splitEditorProps{
			Txn:          editorTxn,
			Categories:   app.Categories(),
			Members:      app.Members(),
			OnSave:       save,
			Pending:      saving.Get(),
			FooterFormID: SplitModalFormID,
			OnCancel:     props.OnDone,
			OnReceiptSplit: func(id string) {
				startReceiptSplitFlow(app, txn)
			},
			DraftGeneration: draftGeneration.Get(),
		}),
	)
}

func postTransactionSplitNotice(hasSplits bool) {
	if hasSplits {
		uistate.PostNotice(uistate.T("splitEditor.r4Saved"), false)
		return
	}
	uistate.PostNotice(uistate.T("splitEditor.cleared"), false)
}
