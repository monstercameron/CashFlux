// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBUDQ61EditorDraftSurvivesTrackingAndFormulaDisclosures(t *testing.T) {
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	cats := state.NewGlobalAtom("budgets:catsEdit", "")
	cats.Set("")
	t.Cleanup(func() { cats.Set("") })
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetEditForm, BudgetEditFormProps{BudgetID: b.ID, Mode: uistate.BudgetEditModeEdit}))
	f.InputByID("budget-edit-name", "Household travel")
	byTestID(f, "input", "budget-edit-limit").Input("175.25")
	byTestID(f, "button", "budget-edit-advanced").Click()
	f.Stabilize()
	byTestID(f, "button", "budget-edit-open-cats").Click()
	f.Stabilize()
	back := byTestID(f, "button", "budget-cats-back")
	if back == nil {
		t.Fatal("tracking page did not open")
	}
	back.Click()
	f.Stabilize()
	if name := f.ByID("budget-edit-name"); name == nil || name.Attr("data-cf-value") != "Household travel" {
		t.Fatalf("name draft = %v", name)
	}
	if limit := byTestID(f, "input", "budget-edit-limit"); limit == nil || limit.Attr("data-cf-value") != "175.25" {
		t.Fatalf("limit draft = %v", limit)
	}
	if toggle := byTestID(f, "button", "budget-edit-advanced"); toggle == nil || toggle.Attr("aria-expanded") != "true" {
		t.Fatal("formula disclosure lost its state")
	}
	if stored := app.Budgets()[0]; stored.Limit.Amount != b.Limit.Amount || stored.Name != b.Name {
		t.Fatal("navigation saved the uncommitted draft")
	}
}

func TestBUDQ61RowDraftAndDetailsSurviveDensitySwitch(t *testing.T) {
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	props := budgetRowProps{Status: budgeting.Status{Budget: b, Limit: b.Limit, Spent: money.Zero("USD"), Remaining: b.Limit}, Category: "Transportation", OnDelete: func(string) {}}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, props))
	byTestID(f, "button", "budget-details-"+b.ID).Click()
	f.Stabilize()
	byTestID(f, "button", "budget-limit-btn-"+b.ID).Click()
	f.Stabilize()
	byTestID(f, "input", "budget-limit-input-"+b.ID).Input("175.25")
	f.Stabilize()
	props.Compact = true
	f.Render(ui.CreateElement(BudgetRow, props))
	f.Stabilize()
	props.Compact = false
	f.Render(ui.CreateElement(BudgetRow, props))
	f.Stabilize()
	if details := byTestID(f, "button", "budget-details-"+b.ID); details == nil || details.Attr("aria-expanded") != "true" {
		t.Fatal("details disclosure lost state across density switch")
	}
	if draft := byTestID(f, "input", "budget-limit-input-"+b.ID); draft == nil || draft.Attr("data-cf-value") != "175.25" {
		t.Fatalf("inline limit draft = %v", draft)
	}
	if got := app.Budgets()[0].Limit.Amount; got != b.Limit.Amount {
		t.Fatalf("density switch persisted limit %d", got)
	}
}
