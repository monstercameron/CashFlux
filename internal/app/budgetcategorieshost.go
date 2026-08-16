// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// BudgetCategoriesHost mounts the "tracked categories" flip modal at the shell root. It
// reads the budgets categories-edit atom and renders the modal when a budget is
// targeted. Mounting here — not inside a bento tile — keeps the fixed modal clear of the
// tile transform that would clip it. The Back body owns its own Save/Cancel (NoFooter).
func BudgetCategoriesHost() uic.Node {
	open := uistate.UseBudgetCategoriesEdit()
	// C521: when the budget editor is open, the picker renders as a page INSIDE it
	// rather than as a second panel — so this host stands down and there is exactly
	// one modal, one Escape and one backdrop. Other entry points (a budget row's
	// "Edit tracking") still get the standalone panel.
	editOpen := uistate.UseBudgetEdit().Get().ID != ""
	if open.Get() == "" || editOpen {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("budgets.catsTitle"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { open.Set("") },
		Back:      uic.CreateElement(screens.BudgetCategoriesBody, struct{}{}),
	})
}
