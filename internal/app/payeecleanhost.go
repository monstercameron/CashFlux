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

// PayeeCleanHost mounts the per-transaction payee-cleanup modal (SM-1) at the shell
// root. It reads the payee-clean atom (the target transaction id) and renders the flip
// modal when set. The body is a <form> (screens.PayeeCleanFormID); the FlipPanel's
// standard pinned footer supplies Cancel + Save (Save = a native submit for that form),
// so the buttons stay fixed at the bottom while the body scrolls — a valid submit
// applies a payee alias for all charges (or a one-off rename) and closes. Mounting at
// the shell root keeps the fixed panel clear of the tile transforms that would clip it.
func PayeeCleanHost() uic.Node {
	open := uistate.UsePayeeClean()
	if open.Get() == "" {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title: uistate.T("payeeClean.title"),
		Width: uiw.FlipSmallW,
		// A touch taller than FlipSmallH so the rename-history lineage is visible without
		// scrolling; the pinned footer + scrolling body still handle a long history.
		Height:       "min(90vh, 560px)",
		FormID:       screens.PayeeCleanFormID,
		SaveLabel:    uistate.T("payeeClean.save"),
		SaveTestID:   "payeeclean-save",
		CancelTestID: "payeeclean-cancel",
		OnClose:      func() { uistate.ClosePayeeClean() },
		Back:         uic.CreateElement(screens.PayeeCleanBody, struct{}{}),
	})
}
