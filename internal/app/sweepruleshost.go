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

// SweepRulesHost is the shell-root flip modal for the AC7 sweep-rules manager:
// list the household's surplus-sweep rules and add new ones. Mounted at the shell
// root so the fixed modal centers on the viewport. The form has no footer Save
// (each add persists immediately); OnClose clears the atom.
func SweepRulesHost() uic.Node {
	open := uistate.UseSweepRulesOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	// Compact modal sized to the short add-form (FlipSmall), and a single Close button:
	// each "Add sweep rule" persists immediately, so there is nothing to Save/Cancel —
	// Close just dismisses. This replaces the tall NoFooter panel that left a large empty
	// void below the content.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("acctSweepCfg.title"),
		Width:     uiw.FlipSmallW,
		Height:    uiw.FlipSmallH,
		CloseOnly: true,
		OnClose:   closeModal,
		Back:      uic.CreateElement(screens.SweepRulesForm, screens.SweepRulesFormProps{OnDone: closeModal}),
	})
}
