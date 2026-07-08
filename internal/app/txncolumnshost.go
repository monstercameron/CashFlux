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

// TxnColumnsHost is mounted at the shell root (beside the other modal hosts). It
// reads the transactions "columns modal open" atom and renders the show/hide-columns
// flip modal when it is set. Rendering here — not inside the transactions toolbar
// tile — keeps the fixed modal clear of the tile's CSS transform, which would
// otherwise mis-position and clip it.
func TxnColumnsHost() uic.Node {
	open := uistate.UseTxnColsModalOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("transactions.columnsTitle"),
		Width:     "360px",
		Height:    "420px",
		CloseOnly: true,
		OnClose:   func() { open.Set(false) },
		Back:      uic.CreateElement(screens.TxnColumnsBody, struct{}{}),
	})
}
