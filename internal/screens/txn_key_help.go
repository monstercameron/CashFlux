// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sync/atomic"

	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

const txnKeyHelpOpenAtomID = "transactions:keyHelpOpen"

var txnKeyHelpOpenAtom state.Atom[bool]
var txnKeyHelpListenerCount atomic.Int32

func txnKeyHelpListenerCountForTest() int32 { return txnKeyHelpListenerCount.Load() }

// SetTxnKeyHelpOpen opens or closes the shell-mounted transaction shortcut guide.
func SetTxnKeyHelpOpen(open bool) {
	txnKeyHelpOpenAtom.Set(open)
}

// TxnKeyHelpHost renders the transaction keyboard guide in the standard dialog
// panel. It is mounted at the shell root so the panel is not clipped by a tile.
func TxnKeyHelpHost() uic.Node {
	open := state.UseAtom(txnKeyHelpOpenAtomID, false)
	txnKeyHelpOpenAtom = open
	uic.UseEffect(func() func() {
		if !open.Get() {
			return nil
		}
		// FlipPanel installs one document keydown and one click listener while
		// mounted. Mirror that lifetime as a test seam for repeated opens.
		txnKeyHelpListenerCount.Add(2)
		return func() { txnKeyHelpListenerCount.Add(-2) }
	}, open.Get())
	if !open.Get() {
		return Fragment()
	}
	return ui.FlipPanel(ui.FlipPanelProps{
		Title:     uistate.T("txnkeys.title"),
		Width:     ui.FlipSmallW,
		Height:    ui.FlipMediumH,
		CloseOnly: true,
		OnClose:   func() { SetTxnKeyHelpOpen(false) },
		Back:      txnKeyHelpBody(),
	})
}

func txnKeyHelpBody() uic.Node {
	keys := []struct{ key, label string }{
		{"txnkeys.keySearch", "txnkeys.search"},
		{"txnkeys.keyNext", "txnkeys.next"},
		{"txnkeys.keyPrevious", "txnkeys.previous"},
		{"txnkeys.keySelect", "txnkeys.select"},
		{"txnkeys.keyEdit", "txnkeys.edit"},
		{"txnkeys.keyCategory", "txnkeys.category"},
		{"txnkeys.keyReview", "txnkeys.review"},
		{"txnkeys.keyHelp", "txnkeys.helpClose"},
	}
	rows := make([]any, 0, len(keys)*2)
	for _, item := range keys {
		rows = append(rows,
			Kbd(css.Class("txn-key-help-key"), uistate.T(item.key)),
			Span(css.Class("txn-key-help-label"), uistate.T(item.label)),
		)
	}
	listArgs := []any{
		css.Class("txn-key-help"),
		Attr("id", "txn-key-help"),
		Attr("data-testid", "txn-key-help"),
		Attr("aria-label", uistate.T("txnkeys.title")),
	}
	rowArgs := append([]any{css.Class("txn-key-help-list")}, rows...)
	listArgs = append(listArgs, Div(rowArgs...))
	return Div(listArgs...)
}
