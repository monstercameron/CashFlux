// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func br2edMenuFixture(isOver bool) ui.Node {
	return ui.CreateElement(func(struct{}) ui.Node {
		menuOpen := ui.UseState(true)
		dummy := ui.UseEvent(func() {})
		status := budgeting.Status{Budget: domain.Budget{ID: "br2ed", Name: "Groceries"}}
		kebab, _ := budgetRowActions(budgetRowActionsProps{
			isOver: isOver, s: status, openTopup: dummy, menuOpen: menuOpen,
			menuID: "br2ed-menu", title: "Groceries", toggleMenu: dummy, closeMenu: dummy,
			props: budgetRowProps{Compact: true}, openCover: dummy, openEdit: dummy,
			openCategories: dummy, openNotes: dummy, notesLabel: "Notes",
			openFormulas: dummy, releaseUnused: dummy, toggleSnooze: dummy,
			snoozeLabel: uistate.T("budgets.pauseBudgetForMonth"), del: dummy,
		})
		return kebab
	}, struct{}{})
}

func TestBudgetMenuShowsBothMoneyMovesInPriorityOrder(t *testing.T) {
	for _, over := range []bool{true, false} {
		t.Run(map[bool]string{true: "over", false: "near-or-under"}[over], func(t *testing.T) {
			f := render.New(t)
			f.Render(br2edMenuFixture(over))
			raise := byTestID(f, "button", "budget-topup-btn-br2ed")
			cover := byTestID(f, "button", "budget-cover-btn-br2ed")
			if raise == nil || cover == nil {
				t.Fatal("row menu must expose both Raise limit and Cover from another budget")
			}
			if raise.Text() != uistate.T("budgets.actionRaiseLimit") || cover.Text() != uistate.T("budgets.actionCoverFromOtherMenu") {
				t.Fatalf("row money actions = %q, %q", raise.Text(), cover.Text())
			}
			buttons := f.AllByTag("button")
			raiseIndex, coverIndex := -1, -1
			for i, button := range buttons {
				switch button.Attr("data-testid") {
				case "budget-topup-btn-br2ed":
					raiseIndex = i
				case "budget-cover-btn-br2ed":
					coverIndex = i
				}
			}
			if raiseIndex < 0 || coverIndex < 0 || raiseIndex >= coverIndex {
				t.Fatalf("Raise limit index %d, Cover index %d; Raise limit must come first", raiseIndex, coverIndex)
			}
		})
	}
}
