// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/screens"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// BudgetBasisHost mounts the "Income to budget with" modal at the shell root (beside the
// other budget modals). It reads the budgets "basis open" atom and renders the flip
// modal — the income-source picker + budget rules — when set. Configs live in flip modals
// here, not inline on the page; mounting at the shell root keeps the fixed panel clear of
// the tile transform that would otherwise clip it. The body edits a staged DRAFT, so the
// standard Save/Cancel footer applies: Save waits for durable storage before
// closing, while Cancel discards the draft.
func BudgetBasisHost() uic.Node {
	open := uistate.UseBudgetBasisOpen()
	if !open.Get() {
		return Fragment()
	}
	return uic.CreateElement(budgetBasisDialog, struct{}{})
}

const budgetBasisFormID = "budget-basis-form"

func budgetBasisDialog(_ struct{}) uic.Node {
	open := uistate.UseBudgetBasisOpen()
	draft := uistate.UseBudgetBasisDraft()
	saving := uic.UseState(false)
	saveFailed := uic.UseState(false)
	save := uic.UseEvent(Prevent(func() {
		if saving.Get() {
			return
		}
		app := appstate.Default
		if app == nil {
			saveFailed.Set(true)
			return
		}
		saveFailed.Set(false)
		if err := uistate.CommitBudgetBasisDraft(draft.Get()); err != nil {
			saveFailed.Set(true)
			return
		}
		saving.Set(true)
		app.RequestDurable(func(result appstate.DurableResult) {
			saving.Set(false)
			if result.Err != nil {
				saveFailed.Set(true)
				return
			}
			open.Set(false)
		})
	}))
	close := func() {
		if !saving.Get() {
			open.Set(false)
		}
	}
	// The native submit footer keeps the dialog open until the dataset writer
	// confirms this preference change reached durable storage.
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:        uistate.T("budgets.basisModalTitle"),
		Width:        uiw.FlipMediumW,
		Height:       uiw.FlipMediumH,
		FormID:       budgetBasisFormID,
		SaveDisabled: saving.Get(),
		OnClose:      close,
		Back: Form(Attr("id", budgetBasisFormID), OnSubmit(save),
			uic.CreateElement(screens.BudgetBasisBody, struct{}{}),
			If(saveFailed.Get(), P(css.Class("form-err"), Attr("role", "alert"), uistate.T("cloud1.saveFailed")))),
	})
}
