// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBud21FuturePeriodUsesPlanningStatusWithoutAlarms(t *testing.T) {
	app := bud2TestApp(t)
	budget := domain.Budget{
		ID: "b-future", Name: "Future plan", CategoryID: "c-food", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(950000, "USD"),
	}
	if err := app.PutBudget(budget); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	configuredPrefs := uistate.LoadPrefs()
	configuredPrefs.BudgetIncomeMode = "fixed"
	configuredPrefs.MonthlyIncomeMinor = 590000
	month := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.Local)
	window := period.NewWindow(period.Month, month, configuredPrefs.WeekStartWeekday())
	f := render.New(t)
	f.Render(ui.CreateElement(bud21FutureSummaryHarness, bud21FutureSummaryProps{
		App: app, Window: window, Prefs: configuredPrefs,
	}))

	sentence := byTestID(f, "span", "budgets-status-sentence")
	if sentence == nil {
		t.Fatal("future-period status sentence did not render")
	}
	if got, want := sentence.Text(), "Planned $9,500.00 · expected income $5,900.00"; got != want {
		t.Errorf("future-period status = %q, want %q", got, want)
	}
	if got := byTestID(f, "div", "budgets-income-alloc"); got != nil {
		t.Errorf("future period rendered income allocation alarm: %q", got.Text())
	}
	if got := byTestID(f, "div", "budgets-funded-callout"); got != nil {
		t.Errorf("future period rendered unfunded alarm: %q", got.Text())
	}
}

func TestBud21BudgetListPrecedesFutureProjectionInDOMOrder(t *testing.T) {
	_ = bud2TestApp(t)
	month := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.Local)
	f := render.New(t)
	f.Render(ui.CreateElement(bud21BudgetSurfaceHarness, bud21SurfaceProps{
		Window: period.NewWindow(period.Month, month, uistate.LoadPrefs().WeekStartWeekday()),
	}))

	listIndex, projectionIndex := -1, -1
	for i, node := range f.AllByTag("div") {
		switch node.Attr("data-widget") {
		case "budget-list":
			listIndex = i
		case "budget-future":
			projectionIndex = i
		}
	}
	if listIndex < 0 || projectionIndex < 0 {
		t.Fatalf("budget list/projection widgets missing: list index %d, projection index %d; diagnostics %v",
			listIndex, projectionIndex, f.BuildDiagnostics())
	}
	if listIndex >= projectionIndex {
		t.Errorf("budget list DOM position %d must precede future projection position %d", listIndex, projectionIndex)
	}
}

type bud21FutureSummaryProps struct {
	App    *appstate.App
	Window period.Window
	Prefs  prefs.Prefs
}

func bud21FutureSummaryHarness(props bud21FutureSummaryProps) ui.Node {
	periodAtom := uistate.UsePeriod()
	currentWindow := periodAtom.Get()
	if !currentWindow.From.Equal(props.Window.From) || !currentWindow.To.Equal(props.Window.To) {
		periodAtom.Set(props.Window)
	}
	prefsAtom := uistate.UsePrefs()
	currentPrefs := prefsAtom.Get()
	if currentPrefs.BudgetIncomeMode != props.Prefs.BudgetIncomeMode || currentPrefs.MonthlyIncomeMinor != props.Prefs.MonthlyIncomeMinor {
		prefsAtom.Set(props.Prefs)
	}
	return ui.CreateElement(budgetSummaryWidget, budgetSummaryProps{App: props.App})
}

type bud21SurfaceProps struct {
	Window period.Window
}

func bud21BudgetSurfaceHarness(props bud21SurfaceProps) ui.Node {
	periodAtom := uistate.UsePeriod()
	currentWindow := periodAtom.Get()
	if !currentWindow.From.Equal(props.Window.From) || !currentWindow.To.Equal(props.Window.To) {
		periodAtom.Set(props.Window)
	}
	return Budgets()
}
