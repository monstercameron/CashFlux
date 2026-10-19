// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func br3dlgCoverActions(_ struct{}) ui.Node {
	status := budgeting.Status{Budget: domain.Budget{ID: "groceries", Name: "Groceries"}, State: budgeting.StateOver, Limit: money.New(45000, "USD")}
	dummy := ui.UseEvent(Prevent(func() {}))
	menuOpen := ui.UseState(true)
	periodNotes := budgetRowPeriodNotes(false, budgetRowProps{}, status, dummy)
	_, actions := budgetRowActions(budgetRowActionsProps{
		isOver: true, s: status, openTopup: dummy, menuOpen: menuOpen, menuID: "br3dlg-menu",
		title: "Groceries", toggleMenu: dummy, closeMenu: dummy, openCover: dummy, openEdit: dummy,
		openCategories: dummy, notesLabel: "Notes", openNotes: dummy, openFormulas: dummy,
		releaseUnused: dummy, toggleSnooze: dummy, snoozeLabel: "Pause", del: dummy,
		coverBtn: periodNotes.coverBtn,
	})
	return actions
}

func TestBR3DLGRowMenuCardAndSingleBudgetDialogCopyAgree(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(br3dlgCoverActions, struct{}{}))

	const action = "Cover from another budget"
	var matching int
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "budget-cover-btn-groceries" {
			matching++
			if button.Text() != action {
				t.Errorf("rendered cover action = %q, want %q", button.Text(), action)
			}
		}
	}
	if matching != 2 {
		t.Fatalf("found %d budget cover buttons, want the card button and row-menu button", matching)
	}
	if got := uistate.T("br3dlg.coverBudgetTitle", "Groceries"); got != "Cover Groceries from another budget" {
		t.Errorf("single-budget dialog title = %q", got)
	}
}
