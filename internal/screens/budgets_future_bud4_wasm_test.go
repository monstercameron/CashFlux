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
	sh "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestFutureBudgetsProjectionStartsCollapsedAndUsesPlanIncome(t *testing.T) {
	app := guardTestApp(t)
	configuredPrefs := uistate.LoadPrefs()
	configuredPrefs.BudgetIncomeMode = "fixed"
	configuredPrefs.MonthlyIncomeMinor = 590000
	budget := quickFillFixtureBudget()
	budget.Limit = money.New(120000, "USD")
	if err := app.PutBudget(budget); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	month := time.Date(time.Now().Year(), time.Now().Month()+1, 1, 0, 0, 0, 0, time.Local)
	if err := app.PutRecurring(domain.Recurring{
		ID: "rent", Label: "Rent", Amount: money.New(-360700, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: month.AddDate(0, 0, 2),
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(budgetFutureHarness, budgetFutureHarnessProps{
		App: app, Prefs: configuredPrefs,
		Window: period.NewWindow(period.Month, month, configuredPrefs.WeekStartWeekday()),
	}))

	toggle := byTestID(f, "button", "budgets-future-toggle")
	if toggle == nil {
		t.Fatalf("future projection did not render for a future month (placeholder=%v, diagnostics=%v)",
			byTestID(f, "div", "budget-future-ph"), f.BuildDiagnostics())
	}
	if got := toggle.Attr("aria-expanded"); got != "false" {
		t.Errorf("future projection aria-expanded = %q, want false by default", got)
	}
	if got, want := toggle.Text(), "Projected for "+month.Format("Jan 2006")+" · 1 bill expected · $3,607.00"; got != want {
		t.Errorf("collapsed projection summary = %q, want %q", got, want)
	}
	if rows := nodesByTestID(f, "budgets-future-row"); len(rows) != 0 {
		t.Fatalf("collapsed projection rendered %d recurring rows", len(rows))
	}

	toggle.Click()
	f.Stabilize()
	planning := byTestID(f, "p", "budgets-future-planning")
	if planning == nil {
		t.Fatal("expanded future projection has no neutral planning message")
	}
	income := byTestID(f, "div", "budgets-future-stat-expectedIncome")
	if income == nil {
		t.Fatal("projection has no expected-income figure")
	}
	if got := income.Text(); got != "Expected income $5,900.00" {
		t.Errorf("projection income figure = %q, want expected income matching plan basis ($5,900.00)", got)
	}
	if got := planning.Text(); got != "Planned $1,200.00 · expected income $5,900.00" {
		t.Errorf("future planning message = %q", got)
	}
	if received := byTestID(f, "p", "budgets-future-received"); received == nil || received.Text() != "Received so far: $0.00" {
		t.Errorf("received-income explanation = %v", received)
	}
	if got, want := len(nodesByTestID(f, "budgets-future-row")), 1; got != want {
		t.Errorf("expanded projection rows = %d, want %d", got, want)
	}
}

type budgetFutureHarnessProps struct {
	App    *appstate.App
	Window period.Window
	Prefs  prefs.Prefs
}

func budgetFutureHarness(props budgetFutureHarnessProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	open := ui.UseState(false)
	toggle := ui.UseEvent(sh.Prevent(func() { open.Set(!open.Get()) }))
	return renderBudgetFutureWidget(budgetSummaryProps{App: props.App}, props.Window,
		time.Now(), "", props.Prefs, open.Get(), toggle)
}

func nodesByTestID(f *render.Fixture, id string) []*render.QueryNode {
	var nodes []*render.QueryNode
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-testid") == id {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func TestIsFuturePeriodUsesViewedWindowStart(t *testing.T) {
	now := time.Date(2026, time.October, 18, 12, 0, 0, 0, time.Local)
	tests := []struct {
		name string
		when time.Time
		want bool
	}{
		{name: "past", when: time.Date(2026, time.September, 1, 0, 0, 0, 0, time.Local)},
		{name: "current", when: now},
		{name: "future", when: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.Local), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := period.NewWindow(period.Month, tt.when, uistate.LoadPrefs().WeekStartWeekday())
			if got := isFuturePeriod(w, now); got != tt.want {
				t.Errorf("isFuturePeriod(%s) = %t, want %t", tt.name, got, tt.want)
			}
		})
	}
}
