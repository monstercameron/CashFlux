// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestRecurringBillsExcludeIncomeAndSplitBudgetCoverage(t *testing.T) {
	app := guardTestApp(t)
	if err := app.PutBudget(domain.Budget{
		ID: "transport-budget", Name: "Transportation", CategoryID: "c-transport",
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly,
		Limit: money.New(50000, "USD"),
	}); err != nil {
		t.Fatal(err)
	}
	for _, recurring := range []domain.Recurring{
		{ID: "gas", Label: "Gas", CategoryID: "c-gas", Amount: money.New(-18000, "USD"), Cadence: domain.CadenceMonthly},
		{ID: "streaming", Label: "Streaming", CategoryID: "c-streaming", Amount: money.New(-1200, "USD"), Cadence: domain.CadenceMonthly},
		{ID: "paycheck", Label: "Paycheck (net)", CategoryID: "c-income", Amount: money.New(470000, "USD"), Cadence: domain.CadenceMonthly},
	} {
		if recurring.ID == "streaming" {
			if err := app.PutCategory(domain.Category{ID: "c-streaming", Name: "Streaming & apps", Kind: domain.KindExpense}); err != nil {
				t.Fatal(err)
			}
		}
		if err := app.PutRecurring(recurring); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	f.Render(ui.CreateElement(recurringBillsHarness, budgetSummaryProps{App: app}))
	toggle := byTestID(f, "button", "budgets-recurring-toggle")
	if toggle == nil || !strings.Contains(toggle.Text(), "Recurring bills (all categories)") {
		t.Fatalf("panel title = %v, want recurring bills across all categories", toggle)
	}
	toggle.Click()
	f.Stabilize()
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Text(), "Paycheck") {
			t.Fatal("income appeared in recurring bills")
		}
	}
	if got := byTestID(f, "span", "budgets-recurring-covered-total"); got == nil || got.Text() != "$180.00" {
		t.Errorf("covered monthly total = %v, want $180.00", got)
	}
	if got := byTestID(f, "span", "budgets-recurring-unbudgeted-total"); got == nil || got.Text() != "($12.00/mo)" {
		t.Errorf("unbudgeted monthly total = %v, want ($12.00/mo)", got)
	}
	if action := byTestID(f, "button", "budget-this-recurring-c-streaming"); action == nil {
		t.Fatal("unbudgeted category has no Budget this action")
	}
	if action := byTestID(f, "button", "budget-this-recurring-c-gas"); action != nil {
		t.Fatal("covered category unexpectedly offers Budget this")
	}
}

func recurringBillsHarness(props budgetSummaryProps) ui.Node {
	return budgetRecurringWidget(props)
}

func TestAnnualPlanHintNamesCalendarYear(t *testing.T) {
	if got := uistate.T("budgets.planYearStableHint", 2026, "$39,190.00"); got != "2026 plan · $39,190.00" {
		t.Fatalf("annual plan hint = %q", got)
	}
	app := guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetAnnualGrid, budgetAnnualGridProps{
		Budgets: app.Budgets(), Rates: budgetRatesForTest(), Now: time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC),
	}))
	button := byTestID(f, "button", "budget-annualgrid-toggle")
	if button == nil || !strings.Contains(button.Text(), "2026 plan") {
		t.Fatalf("collapsed annual plan label = %v, want calendar-year plan label", button)
	}
}

func budgetRatesForTest() currency.Rates { return currency.Rates{Base: "USD"} }
