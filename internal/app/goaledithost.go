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

	// Standard sizes: full edit uses Medium, the short contribute form uses Small.
	title, width, height := uistate.T("goals.editTitle"), uiw.FlipMediumW, uiw.FlipMediumH
	if e.Mode == uistate.GoalEditModeContribute {
		title, width, height = uistate.T("goals.contributeTitle"), uiw.FlipSmallW, uiw.FlipSmallH
	}

	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     title,
		Width:     width,
		Height:    height,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   closeModal,
		Back:      uic.CreateElement(screens.GoalEditForm, screens.GoalEditFormProps{GoalID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
