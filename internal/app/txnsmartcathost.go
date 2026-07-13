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

// TxnSmartCatHost mounts the Smart+ categorization review modal at the shell root
// (beside the other modal hosts). It reads the transactions "smart categorize"
// atom and renders the flip modal when set. Mounting here — not inside a bento
// tile — keeps the fixed modal clear of the tile transform that would clip it.
func TxnSmartCatHost() uic.Node {
	open := uistate.UseTxnSmartCatOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("smartcat.title"),
		Width:     uiw.FlipMediumW, // review list of suggestions
		Height:    uiw.FlipMediumH,
		CloseOnly: true,
		OnClose:   func() { open.Set(false) },
		Back:      uic.CreateElement(screens.TxnSmartCatBody, struct{}{}),
	})
}
