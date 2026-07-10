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

// AutoBudgetHost mounts the "Auto budget" review modal at the shell root (beside the
// other modal hosts). It reads the budgets "auto open" atom and renders the flip modal
// when set. Mounting here — not inside a bento tile — keeps the fixed modal clear of the
// tile transform that would clip it. The Back body owns its own Save/Cancel buttons
// (NoFooter), so the modal isn't double-chromed.
func AutoBudgetHost() uic.Node {
	open := uistate.UseBudgetAutoOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("budgets.autoTitle"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { open.Set(false) },
		Back:      uic.CreateElement(screens.AutoBudgetBody, struct{}{}),
	})
}
