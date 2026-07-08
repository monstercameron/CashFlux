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

// TxnLinkHost mounts the payment-link flip modal at the shell root (beside the other
// modal hosts). It reads the transactions "link target" atom and renders the flip
// modal when a transaction is targeted. Mounting here — not inside a bento tile —
// keeps the fixed modal clear of the tile transform that would clip it. The Back body
// owns its own Save/Cancel buttons (NoFooter), so the modal isn't double-chromed.
func TxnLinkHost() uic.Node {
	target := uistate.UseTxnLinkTarget()
	if target.Get().TxnID == "" {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("txnlink.title"),
		Width:    "420px",
		Height:   "500px",
		NoFooter: true,
		OnClose:  func() { target.Set(uistate.TxnLinkTarget{}) },
		Back:     uic.CreateElement(screens.TxnLinkBody, struct{}{}),
	})
}
