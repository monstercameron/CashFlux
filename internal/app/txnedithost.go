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
// The FlipPanel is configured with CloseOnly: true so its footer is a single
// "Close" button — the edit form owns its own Save/Delete/Cancel buttons and
// inline error. On a successful save (or delete) the form calls OnDone (which
// clears the atom), closing the modal. On a validation error the form stays open.
func TxnEditHost() uic.Node {
	edit := uistate.UseTxnEdit()
	id := edit.Get()
	if id == "" {
		return Fragment()
	}
	close := func() { uistate.SetTxnEdit("") }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("transactions.editTitle"),
		Width:    uiw.FlipMediumW,
		Height:   uiw.FlipMediumH,
		NoFooter: true, // the form owns its Delete / Cancel / Save bar (sticky-pinned) — no redundant Close
		OnClose:  close,
		Back:     uic.CreateElement(screens.TransactionEditForm, screens.TransactionEditFormProps{TxnID: id, OnDone: close}),
	})
}
