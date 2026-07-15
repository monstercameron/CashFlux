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
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        uistate.T("acctSweepCfg.title"),
		Width:        uiw.FlipMediumW,
		Height:       uiw.FlipMediumH,
		NoFooter:     true,
		CancelTestID: "sweep-cfg-close",
		OnClose:      closeModal,
		Back:         uic.CreateElement(screens.SweepRulesForm, screens.SweepRulesFormProps{OnDone: closeModal}),
	})
}
