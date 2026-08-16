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
	// C566: the footer's Save mirrors the draft's validity. The editor publishes its
	// verdict into this atom, so a split with an unfinished line shows a disabled Save
	// instead of an enabled one that can only fail. Read UNCONDITIONALLY, above the
	// early return — a hook behind a condition drifts the hook order (GWC rule).
	readyAtom := uistate.UseTxnSplitReady()
	id := split.Get()
	if id == "" {
		return Fragment()
	}
	// Closing resets the verdict, so the next split opens on a live Save rather than
	// inheriting the last draft's disabled state for a frame.
	close := func() { readyAtom.Set(true); uistate.SetTxnSplit("") }
	ready := readyAtom.Get()
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        uistate.T("splitEditor.title"),
		Width:        uiw.FlipMediumW,
		Height:       uiw.FlipMediumH,
		FormID:       screens.SplitModalFormID,
		SaveLabel:    uistate.T("splitEditor.save"),
		SaveTestID:   "split-save",
		SaveDisabled: !ready,
		OnClose:      close,
		Back:         uic.CreateElement(screens.TransactionSplitForm, screens.TransactionSplitFormProps{TxnID: id, OnDone: close}),
	})
}
