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

// AllocProfileHost is the shell-root flip modal for the /allocate strategy (split mode, ranking
// profile, buffer/cap, criterion weights, saved profiles). Driven by the invest… no — the
// alloc-profile-open atom. Mounted at the shell root so the modal's position:fixed centres on the
// viewport (bento tiles carry transforms that would offset it). The form owns Done/Cancel and
// clears the atom via OnDone. The strategy lives in shared atoms so the ranked plan behind the
// modal re-ranks live as it's tuned.
func AllocProfileHost() uic.Node {
	open := uistate.UseAllocProfileOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    uistate.T("allocate.strategyTitle"),
		Width:    "560px",
		Height:   "min(90vh, 640px)",
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.AllocProfileForm, screens.AllocProfileFormProps{OnDone: closeModal}),
	})
}
