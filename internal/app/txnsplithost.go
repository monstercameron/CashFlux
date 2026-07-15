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

// TxnSplitHost is mounted at the shell root (beside TxnEditHost). It reads the
// TxnSplit atom and renders the split-into-categories editor inside a FlipPanel
// modal, so a big purchase (e.g. one Amazon charge) can be carved into per-budget
// category amounts from the row's ⋯ menu. When the atom is empty no overlay is
// shown.
//
// NoFooter: the SplitEditor owns its own Save/Clear buttons and inline error; a
// successful save calls OnDone (clearing the atom), closing the modal, while a
// validation error keeps it open.
func TxnSplitHost() uic.Node {
	split := uistate.UseTxnSplit()
	id := split.Get()
	if id == "" {
		return Fragment()
	}
	close := func() { uistate.SetTxnSplit("") }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("splitEditor.title"),
		Width:    uiw.FlipMediumW,
		Height:   uiw.FlipMediumH,
		NoFooter: true,
		OnClose:  close,
		Back:     uic.CreateElement(screens.TransactionSplitForm, screens.TransactionSplitFormProps{TxnID: id, OnDone: close}),
	})
}
