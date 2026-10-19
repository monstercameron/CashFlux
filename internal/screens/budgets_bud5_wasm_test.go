// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBudgetAddStartsBlankAndFocusesFirstRequiredField(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{}))

	name := f.ByID("budget-add")
	if name == nil {
		t.Fatal("the required Name field is missing")
	}
	if got := name.Attr("autofocus"); got != "autofocus" {
		t.Fatalf("blank add form autofocus = %q, want Name", got)
	}
	category := byTestID(f, "button", "budget-existing-cat")
	if category == nil || category.Text() != uistate.T("budgets.chooseCategory") {
		t.Fatalf("blank category selection = %v, want the choose-category prompt", category)
	}
	if !isDisabled(byTestID(f, "button", "budget-add-submit")) {
		t.Fatal("Add budget is enabled before a category has been chosen")
	}
	f.Cleanup()

	seeded := render.New(t)
	seeded.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{
		Seed: uistate.BudgetAddSeed{Name: "Transportation", CategoryID: "c-transport"},
	}))
	if category := byTestID(seeded, "button", "budget-existing-cat"); category == nil || category.Text() != "Transportation" {
		t.Fatalf("seeded category = %v, want Transportation", category)
	}
	if target := byTestID(seeded, "input", "budget-add-limit"); target == nil || target.Attr("autofocus") != "autofocus" {
		t.Fatal("when the seeded name and category are present, focus must move to the empty limit")
	}
	seeded.Cleanup()

	chip := render.New(t)
	chip.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{
		Seed: uistate.BudgetAddSeed{Name: "Transportation", CategoryID: "c-transport", LimitMajor: "74.25"},
	}))
	if category := byTestID(chip, "button", "budget-existing-cat"); category == nil || category.Text() != "Transportation" {
		t.Fatalf("Budget this category prefill = %v, want Transportation", category)
	}
	if got := byTestID(chip, "input", "budget-add-limit").Attr("data-cf-value"); got != "74.25" {
		t.Fatalf("Budget this limit prefill = %v, want 74.25", got)
	}
}

func TestBudgetAddCategoryOptionsDisableExistingPeriodBudgets(t *testing.T) {
	app := guardTestApp(t)
	b := domain.Budget{
		ID: "b-transport", Name: "Transportation", CategoryID: "c-transport",
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Period: domain.PeriodMonthly, Limit: money.New(130000, "USD"),
	}
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	options := budgetAddCategoryOptions(app.Categories(), app.Budgets(), string(domain.PeriodMonthly), domain.GroupOwnerID)
	for _, option := range options {
		if option.Value != "c-transport" {
			continue
		}
		if !option.Disabled {
			t.Fatal("Transportation remains selectable even though it already has a monthly budget")
		}
		if option.Secondary != uistate.T("budgets.categoryAlreadyBudgeted") {
			t.Fatalf("disabled option explanation = %q", option.Secondary)
		}
		return
	}
	t.Fatal("Transportation was omitted instead of shown as already budgeted")
}

func TestBudgetAddCategorySelectionDefaultsName(t *testing.T) {
	for _, tc := range []struct{ current, previous, next, want string }{
		{"", "", "Transportation", "Transportation"},
		{"Transportation", "Transportation", "Groceries", "Groceries"},
		{"My monthly plan", "Transportation", "Groceries", "My monthly plan"},
	} {
		if got := budgetAddNameForCategory(tc.current, tc.previous, tc.next); got != tc.want {
			t.Errorf("budgetAddNameForCategory(%q, %q, %q) = %q, want %q", tc.current, tc.previous, tc.next, got, tc.want)
		}
	}
}

func TestRaiseLimitCopyUsesOneActionName(t *testing.T) {
	if got := uistate.T("budgets.topupTitle"); got != "Raise limit" {
		t.Errorf("dialog title = %q", got)
	}
	if got := uistate.T("budgets.raiseLimit"); got != "Raise limit" {
		t.Errorf("primary action = %q", got)
	}
	period := uistate.T("budgets.raisePeriodOnly", "October 2026")
	if period != "This raises the limit for October 2026 only." {
		t.Errorf("period-only explanation = %q", period)
	}
	if got := uistate.T("budgets.raisePermanently"); got != "This raises the limit permanently." {
		t.Errorf("permanent explanation = %q", got)
	}
}
