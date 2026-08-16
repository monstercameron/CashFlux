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

// TxnHistoryHost is mounted at the shell root (beside TxnSplitHost). It reads
// the TxnHistory atom and renders the per-transaction history panel (#63)
// inside a FlipPanel modal — every recorded change to one transaction, from
// the row's ⋯ menu. Read-only: the footer is just Close.
func TxnHistoryHost() uic.Node {
	hist := uistate.UseTxnHistory()
	id := hist.Get()
	if id == "" {
		return Fragment()
	}
	close := func() { uistate.SetTxnHistory("") }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:  uistate.T("txnhistory.title"),
		Width:  uiw.FlipMediumW,
		Height: uiw.FlipMediumH,
		// C567: the panel reads a transaction's audit trail — there is nothing here to
		// commit. Without CloseOnly the FlipPanel falls through to its standard
		// Cancel/Save footer, so an empty history offered a Save that could only ever
		// do nothing, and a Cancel that implied unsaved edits existed.
		CloseOnly: true,
		OnClose:   close,
		Back:      uic.CreateElement(screens.TxnHistoryPanel, screens.TxnHistoryPanelProps{TxnID: id}),
	})
}
