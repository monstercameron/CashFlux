// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// savedTxnEditScroll holds the ledger pane's scroll offset while the edit modal
// is open; -1 means nothing to restore.
var savedTxnEditScroll = -1.0

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
	// #59 (mobile pass, applies everywhere): opening a row's edit modal must not
	// lose the list's scroll position. Capture the scroll pane's offset when the
	// modal opens and re-assert it after close — the close-time focus restore
	// scrolls the focused row into view, which yanked the pane hundreds of
	// pixels from where the user was. The effect runs on the open/closed
	// transition (hook is unconditional; the early return below only gates
	// rendering).
	openKey := "closed"
	if id != "" {
		openKey = "open"
	}
	uic.UseEffect(func() func() {
		sc := js.Global().Get("document").Call("querySelector", "main.cf-scroll")
		if !sc.Truthy() {
			return nil
		}
		if id != "" {
			savedTxnEditScroll = sc.Get("scrollTop").Float()
			return nil
		}
		if savedTxnEditScroll >= 0 {
			top := savedTxnEditScroll
			savedTxnEditScroll = -1
			sc.Set("scrollTop", top)
			// Focus restoration lands a beat later and scrolls again; one
			// delayed re-assert wins without fighting user scrolling.
			var cb js.Func
			cb = js.FuncOf(func(js.Value, []js.Value) any {
				if s2 := js.Global().Get("document").Call("querySelector", "main.cf-scroll"); s2.Truthy() {
					s2.Set("scrollTop", top)
				}
				cb.Release()
				return nil
			})
			js.Global().Call("setTimeout", cb, 120)
		}
		return nil
	}, openKey)
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
