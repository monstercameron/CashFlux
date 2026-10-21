// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestBV7CHeadlineSpendUsesSelectedMonthOnceAcrossOverlappingBudgets(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	month := period.NewWindow(period.Month, start, time.Monday)
	yearly := domain.Budget{ID: "yearly", CategoryID: "transport-cat", Scope: domain.ScopeShared,
		OwnerID: domain.GroupOwnerID, Period: domain.PeriodYearly, Limit: money.New(330000, "USD")}
	tag := domain.Budget{ID: "tag", TrackedTags: []string{"trip"}, Scope: domain.ScopeShared,
		OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}
	br2sugPutExpense(t, app, "month-spend", start.AddDate(0, 0, 3), -25000)
	br2sugPutExpense(t, app, "year-to-date", start.AddDate(0, -1, 3), -270000)
	var inMonth domain.Transaction
	for _, txn := range app.Transactions() {
		if txn.ID == "month-spend" {
			inMonth = txn
			break
		}
	}
	if inMonth.ID == "" {
		t.Fatal("selected-month transaction was not saved")
	}
	inMonth.Tags = []string{"trip"}
	if err := app.PutTransaction(inMonth); err != nil {
		t.Fatalf("tag selected-month transaction: %v", err)
	}
	if err := app.PutCategory(domain.Category{ID: "home-cat", Name: "Home", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	br2sugPutExpense(t, app, "split-spend", start.AddDate(0, 0, 5), -30000)
	var splitTxn domain.Transaction
	for _, txn := range app.Transactions() {
		if txn.ID == "split-spend" {
			splitTxn = txn
			break
		}
	}
	splitTxn.Splits = []domain.CategorySplit{
		{CategoryID: "transport-cat", Amount: money.New(-10000, "USD")},
		{CategoryID: "home-cat", Amount: money.New(-20000, "USD")},
	}
	if err := app.PutTransaction(splitTxn); err != nil {
		t.Fatalf("PutTransaction(split): %v", err)
	}
	home := domain.Budget{ID: "home", CategoryID: "home-cat", Scope: domain.ScopeShared,
		OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}
	got, issues := budgetSummaryActualSpend(app, []budgeting.Status{{Budget: yearly}, {Budget: tag}, {Budget: home}}, month, false,
		currency.Rates{Base: "USD"}, "USD")
	if got != 55000 || len(issues) != 0 {
		t.Fatalf("selected-month spend = %d, issues %v; want union spend 55000 and no issues", got, issues)
	}
}

func TestBV7CHeadlinePlanUsesViewedLimitHistory(t *testing.T) {
	app := br2sugTestApp(t)
	window := period.NewWindow(period.Month, time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), time.Monday)
	monthly := domain.Budget{ID: "monthly-history", Period: domain.PeriodMonthly, Limit: money.New(45000, "USD"),
		LimitHistory: map[string]int64{"2026-08-01": 38000, "2026-10-01": 45000}}
	yearly := domain.Budget{ID: "yearly-history", Period: domain.PeriodYearly, Limit: money.New(330000, "USD"),
		LimitHistory: map[string]int64{"2026-01-01": 200000, "2026-10-01": 330000}}
	plan := budgetSummaryPlan(budgetView{Base: "USD", Statuses: []budgeting.Status{{Budget: monthly}, {Budget: yearly}}}, app, window)
	if plan.normalizedBudgetTotal != 54667 {
		t.Fatalf("September monthly plan = %d, want history-aware $546.67", plan.normalizedBudgetTotal)
	}
	if monthly.Limit.Amount != 45000 || yearly.Limit.Amount != 330000 {
		t.Fatal("building the historical plan mutated the stored budgets")
	}
}

func TestBV7CHeadlinePlanProratesPartialCalendarWindow(t *testing.T) {
	window := period.NewWindow(period.Week, time.Date(2026, time.March, 8, 0, 0, 0, 0, time.UTC), time.Sunday)
	budget := domain.Budget{ID: "monthly", Period: domain.PeriodMonthly, Limit: money.New(31000, "USD")}
	got, excluded, issues := budgetSummaryWindowPlan([]budgeting.Status{{Budget: budget}}, window, false, currency.Rates{Base: "USD"})
	if got != 7000 || excluded != 0 || len(issues) != 0 {
		t.Fatalf("one week of March plan = %d, excluded %d, issues %v; want 7000 with no exclusions", got, excluded, issues)
	}
}

func TestBV7CCalendarDayCountIgnoresDSTOffsetChange(t *testing.T) {
	start := time.Date(2026, time.March, 8, 0, 0, 0, 0, time.FixedZone("EST", -5*60*60))
	end := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	if got := budgetCalendarDaysBetween(start, end); got != 7 {
		t.Fatalf("calendar-day count = %d, want 7 across the spring clock change", got)
	}
}

func TestBV7CHeadlineUsesSelectedWindowFigureAndNamesWindow(t *testing.T) {
	window := period.NewWindow(period.Month, time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), time.Monday)
	f := render.New(t)
	f.Render(budgetSummaryStatus(budgetSummaryStatusProps{
		v: budgetView{Base: "USD", TotalSpent: 294250}, vw: window,
		normalizedBudgetTotal: 355833, windowSpent: 24250, windowSpentSet: true,
		normalizationOK: true,
	}))
	figure := byTestID(f, "span", "budgets-status-figure")
	if figure == nil || figure.Text() != "$242.50" {
		t.Fatalf("headline figure = %v, want selected-window $242.50", figure)
	}
	label := byTestID(f, "span", "budgets-status-window")
	if label == nil || !strings.Contains(label.Text(), "Oct 2026") {
		t.Fatalf("headline window label = %v, want October 2026", label)
	}
}
