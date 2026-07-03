// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// InvestPoolEditHost is the shell-root flip modal for creating or editing an investment
// pool: a name field + a checkable list of accounts to include. Driven by the
// invest-pool-edit atom ("" = closed, "new" = create, or a pool id = edit). Mounted at the
// shell root so the modal's position:fixed centres on the viewport (bento tiles carry
// transforms that would otherwise offset it). The form owns Save/Cancel and clears the atom
// via OnDone.
func InvestPoolEditHost() uic.Node {
	edit := uistate.UseInvestPoolEditID()
	pid := edit.Get()
	if pid == "" {
		return Fragment()
	}
	closeModal := func() { edit.Set("") }
	title := uistate.T("investments.newPoolTitle")
	if pid != "new" {
		title = uistate.T("investments.editPoolTitle")
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    title,
		Width:    "480px",
		Height:   "560px",
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.InvestPoolForm, screens.InvestPoolFormProps{ID: pid, OnDone: closeModal}),
	})
}
