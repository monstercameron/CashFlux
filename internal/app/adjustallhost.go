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

// AdjustAllHost mounts the "Adjust all budgets" form at the shell root, beside
// the other budget modal hosts (C592).
//
// It has a TITLE, which the control it replaces did not: "Adjust all" opened a
// bare prompt whose only content was a question, so the dialog never said what it
// was. Mounting here — not inside a bento tile — keeps the fixed panel clear of
// the tile transform that would clip it, and the body owns its own Apply/Cancel
// (NoFooter) so the modal is not double-chromed.
func AdjustAllHost() uic.Node {
	open := uistate.UseBudgetAdjustOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("budgets.adjustAllTitleHeading"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { open.Set(false) },
		Back:      uic.CreateElement(screens.AdjustAllBody, struct{}{}),
	})
}
