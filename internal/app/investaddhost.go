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

// InvestAddHost is mounted at the shell root (beside the other edit/add hosts). It reads
// the invest-add-open atom and renders the "Add a security" form inside a FlipPanel modal.
// When the atom is false, no overlay is shown.
//
// Mounting at the shell root — not inside the investments securities tile — is the whole
// point: a tile lives under transformed bento ancestors (.w / the app-enter wrapper carry
// transforms), which would make the modal's position:fixed backdrop resolve against the
// tile instead of the viewport, rendering off-centre. Here there is no transformed
// ancestor, so the modal centres. The form owns its own Add/Cancel (NoFooter) and calls
// OnDone to close (which clears the atom) — fixing the earlier hook-outside-component crash
// from calling UseInvestAddOpen() inside a click handler.
func InvestAddHost() uic.Node {
	open := uistate.UseInvestAddOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("investments.addHoldingTitle"),
		Width:    "560px",
		Height:   "440px",
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.InvestAddForm, screens.InvestAddFormProps{OnDone: closeModal}),
	})
}
