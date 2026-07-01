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

// GoalEditHost is mounted at the shell root (beside BudgetEditHost). It reads the
// goal-editor atom and renders the matching editor (full edit / contribute) inside a
// FlipPanel modal. When the atom is empty no overlay is shown. Mounting at the shell
// root keeps the modal centred on the viewport (a goal card lives under transformed
// bento/tile ancestors, which would push an in-card modal off-centre).
func GoalEditHost() uic.Node {
	edit := uistate.UseGoalEdit()
	e := edit.Get()
	if e.ID == "" || e.Mode == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseGoalEdit() }

	title, width, height := uistate.T("goals.editTitle"), "460px", "560px"
	if e.Mode == uistate.GoalEditModeContribute {
		title, width, height = uistate.T("goals.contributeTitle"), "420px", "340px"
	}

	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:    title,
		Width:    width,
		Height:   height,
		NoFooter: true,
		OnClose:  closeModal,
		Back:     uic.CreateElement(screens.GoalEditForm, screens.GoalEditFormProps{GoalID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
