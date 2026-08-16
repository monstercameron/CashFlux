// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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
// budgetHasNote reports whether the budget already carries a note, so the editor
// can say whether it is about to create one or change one.
func budgetHasNote(id string) bool {
	app := appstate.Default
	if app == nil || id == "" {
		return false
	}
	for _, b := range app.Budgets() {
		if b.ID == id {
			return strings.TrimSpace(b.Notes) != ""
		}
	}
	return false
}

func BudgetEditHost() uic.Node {
	edit := uistate.UseBudgetEdit()
	// Read BEFORE the early return: hooks are positional, so one behind a
	// conditional return shifts every hook after it on the renders that take the
	// other branch.
	childOpen := uistate.UseBudgetCategoriesEdit().Get() != ""
	e := edit.Get()
	if e.ID == "" || e.Mode == "" {
		return Fragment()
	}
	// C521: closing the editor also leaves the tracked-categories page, so
	// reopening never lands on a sub-page the user did not ask for.
	closeModal := func() {
		if childOpen {
			uistate.SetBudgetCategoriesEdit("")
		}
		uistate.CloseBudgetEdit()
	}

	// All budget editors are Medium (the top-up grew a duration picker + optional
	// fund-from-budgets checklist, so it's no longer the short Small form).
	title, width, height := uistate.T("budgets.editTitle"), uiw.FlipMediumW, uiw.FlipMediumH
	switch e.Mode {
	case uistate.BudgetEditModeTopup:
		title = uistate.T("budgets.topupTitle")
	case uistate.BudgetEditModeCover:
		title = uistate.T("budgets.coverModalTitle")
	case uistate.BudgetEditModeNotes:
		// Name the action, and name it by state: the same editor either creates a
		// note or overwrites one, and "Budget notes" said neither. The row's ⋯ item
		// is labelled the same way, so the modal confirms what was clicked.
		title = uistate.T("budgets.notesAdd")
		if budgetHasNote(e.ID) {
			title = uistate.T("budgets.notesEdit")
		}
	case uistate.BudgetEditModeFormulas:
		title = uistate.T("budgets.formulasTitle")
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
