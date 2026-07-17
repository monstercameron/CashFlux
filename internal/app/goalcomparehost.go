// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalCompareHost is mounted at the shell root (beside GoalEditHost). It reads
// the compare-open atom and renders the goal-vs-goal comparison inside a
// FlipPanel modal. When the atom is false no overlay is shown. Mounted at the
// shell root so the fixed backdrop centers on the viewport, like every other
// goal editor.
func GoalCompareHost() uic.Node {
	open := uistate.UseGoalCompareOpen()
	if !open.Get() {
		return Fragment()
	}
	closeModal := func() { open.Set(false) }
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("goalcompare.title"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   closeModal,
		Back: uic.CreateElement(screens.GoalCompareForm, screens.GoalCompareProps{
			App: appstate.Default, OnDone: closeModal,
		}),
	})
}
