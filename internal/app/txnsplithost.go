// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// TxnSplitHost is mounted at the shell root (beside TxnEditHost). It reads the
// TxnSplit atom and renders the split-into-categories editor inside a FlipPanel
// modal, so a big purchase (e.g. one Amazon charge) can be carved into per-budget
// category amounts from the row's ⋯ menu. When the atom is empty no overlay is
// shown.
//
// The editor renders as a body <form> (screens.SplitModalFormID); the FlipPanel's
// standard pinned footer supplies Save (a native submit for that form) and Cancel,
// so Save/Cancel stay fixed at the bottom while the split rows scroll. On a valid
// submit the form calls OnDone (clearing the atom) which closes the modal; a
// validation error keeps it open. "Clear split" remains a body action inside the form.
func TxnSplitHost() uic.Node {
	split := uistate.UseTxnSplit()
	id := split.Get()
	if id == "" {
		return Fragment()
	}
	close := func() { uistate.SetTxnSplit("") }
	// C566: the editor supplies its own pinned Cancel/Save bar (NoFooter + FlushBody),
	// because whether the draft can be saved is a fact only the editor holds — line
	// completeness and the remainder. The panel's standard footer could not read it,
	// so an unfinished split showed a live Save that could only fail validation.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("splitEditor.title"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   close,
		Back:      uic.CreateElement(screens.TransactionSplitForm, screens.TransactionSplitFormProps{TxnID: id, OnDone: close}),
	})
}
