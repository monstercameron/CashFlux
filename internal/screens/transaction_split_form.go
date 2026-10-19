// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
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
		for _, t := range app.Transactions() {
			if t.ID == props.TxnID {
				txn, found = t, true
				break
			}
		}
	}

	// Consume a receipt-derived proposal once so the editor can review its lines.
	proposedNote := ui.UseState("")
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
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	if !found {
		return Fragment()
	}

	// Route local suggestions and receipt proposals through the same editor state.
	useProposal := func(lines []domain.CategorySplit) {
		if len(lines) == 0 {
			return
		}
		t := txn
		t.Splits = lines
		seededTxn.Set(t)
		proposedNote.Set("")
		haveSeed.Set(true)
		draftGeneration.Set(draftGeneration.Get() + 1)
	}

	editorTxn := txn
	if haveSeed.Get() {
		// Keep the live transaction's mutable fields but carry the proposed splits.
		editorTxn = seededTxn.Get()
	}

	save := func(updated domain.Transaction) {
		if err := app.PutTransaction(updated); err != nil {
			uistate.PostNotice(err.Error(), true)
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

	return Fragment(
		// Keep suggestion handlers in their own component.
		ui.CreateElement(splitSuggestHint, splitSuggestProps{Txn: editorTxn, OnUse: useProposal}),
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
			FooterFormID: SplitModalFormID,
			OnCancel:     props.OnDone,
			OnReceiptSplit: func(id string) {
				startReceiptSplitFlow(app, txn)
			},
			DraftGeneration: draftGeneration.Get(),
		}),
	)
}
