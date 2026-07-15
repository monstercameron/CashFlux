// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// TxnEditHost is mounted at the shell root (beside AddHost). It reads the
// TxnEdit atom and renders the transaction edit form inside a FlipPanel modal.
// When the atom is empty no overlay is shown.
//
// The FlipPanel supplies the standard pinned footer via FormID — Cancel plus a
// Save that natively submits the form (the form does the work in OnSubmit and
// closes via OnDone; a validation error keeps it open). Delete lives in the form
// body, apart from the pinned pair. This replaced the form's own in-body button
// row, which floated mid-panel instead of pinning to the bottom.
func TxnEditHost() uic.Node {
	edit := uistate.UseTxnEdit()
	id := edit.Get()
	if id == "" {
		return Fragment()
	}
	close := func() { uistate.SetTxnEdit("") }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:      uistate.T("transactions.editTitle"),
		Width:      uiw.FlipMediumW,
		Height:     uiw.FlipMediumH,
		FormID:     "txn-edit-form",
		SaveTestID: "txn-edit-save",
		OnClose:    close,
		Back:       uic.CreateElement(screens.TransactionEditForm, screens.TransactionEditFormProps{TxnID: id, OnDone: close}),
	})
}
