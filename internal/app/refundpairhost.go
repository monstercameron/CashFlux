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

// RefundPairHost mounts the refund-pairing flip modal at the shell root (beside
// the other modal hosts). It reads the refund-pair target atom and renders the
// picker when a refund transaction is targeted (XC2). Mounting here — not inside
// a bento tile — keeps the fixed modal clear of the tile transform that would
// clip it. The body owns its own Save/Cancel buttons (NoFooter).
func RefundPairHost() uic.Node {
	target := uistate.UseRefundPairTarget()
	if target.Get() == "" {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("txnlinks.pairTitle"),
		Width:    uiw.FlipMediumW,
		Height:   uiw.FlipMediumH,
		NoFooter: true,
		OnClose:  func() { target.Set("") },
		Back:     uic.CreateElement(screens.RefundPairBody, struct{}{}),
	})
}
