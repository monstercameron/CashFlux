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

// BudgetBasisHost mounts the "Income to budget with" modal at the shell root (beside the
// other budget modals). It reads the budgets "basis open" atom and renders the flip
// modal — the income-source picker + budget rules — when set. Configs live in flip modals
// here, not inline on the page; mounting at the shell root keeps the fixed panel clear of
// the tile transform that would otherwise clip it. The body edits a staged DRAFT, so the
// standard Save/Cancel footer applies: Save commits the draft to prefs, Cancel discards.
func BudgetBasisHost() uic.Node {
	open := uistate.UseBudgetBasisOpen()
	draft := uistate.UseBudgetBasisDraft()
	if !open.Get() {
		return Fragment()
	}
	// Keeps the standard pinned Save/Cancel footer (staged draft) — just a standard size.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:   uistate.T("budgets.basisModalTitle"),
		Width:   uiw.FlipMediumW,
		Height:  uiw.FlipMediumH,
		OnSave:  func() { uistate.CommitBudgetBasisDraft(draft.Get()) },
		OnClose: func() { open.Set(false) },
		Back:    uic.CreateElement(screens.BudgetBasisBody, struct{}{}),
	})
}
