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

// BudgetEditHost is mounted at the shell root (beside AccountEditHost). It reads the
// budget-editor atom and renders the matching editor (full edit / top-up) inside a
// FlipPanel modal. When the atom is empty no overlay is shown.
//
// Mounting at the shell root — rather than inside the budget row — is the whole point:
// a row lives under transformed bento/tile ancestors (`.w`, `.card`, the app-enter
// wrapper all carry a transform), which would make the modal's position:fixed backdrop
// resolve against the tile instead of the viewport, so an in-row modal rendered
// off-centre. Here there is no transformed ancestor, so the modal centres on the
// viewport. The editor form owns its own Save/Cancel (NoFooter) and calls OnDone on
// completion, which clears the atom.
func BudgetEditHost() uic.Node {
	edit := uistate.UseBudgetEdit()
	e := edit.Get()
	if e.ID == "" || e.Mode == "" {
		return Fragment()
	}
	closeModal := func() { uistate.CloseBudgetEdit() }

	// Standard sizes: full edit + cover use Medium, the short top-up uses Small.
	title, width, height := uistate.T("budgets.editTitle"), uiw.FlipMediumW, uiw.FlipMediumH
	switch e.Mode {
	case uistate.BudgetEditModeTopup:
		title, width, height = uistate.T("budgets.topupTitle"), uiw.FlipSmallW, uiw.FlipSmallH
	case uistate.BudgetEditModeCover:
		title = uistate.T("budgets.coverModalTitle")
	}

	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     title,
		Width:     width,
		Height:    height,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   closeModal,
		Back:      uic.CreateElement(screens.BudgetEditForm, screens.BudgetEditFormProps{BudgetID: e.ID, Mode: e.Mode, OnDone: closeModal}),
	})
}
