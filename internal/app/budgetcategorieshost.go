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

// BudgetCategoriesHost mounts the "tracked categories" flip modal at the shell root. It
// reads the budgets categories-edit atom and renders the modal when a budget is
// targeted. Mounting here — not inside a bento tile — keeps the fixed modal clear of the
// tile transform that would clip it. The Back body owns its own Save/Cancel (NoFooter).
func BudgetCategoriesHost() uic.Node {
	open := uistate.UseBudgetCategoriesEdit()
	if open.Get() == "" {
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
