// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestBV6FHeadlineUsesActualWindowSpendForEveryCadence(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	budget := domain.Budget{ID: "yearly-splurges", Name: "Splurges", CategoryID: "transport-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodYearly, Limit: money.New(330000, "USD")}
	br2sugPutExpense(t, app, "in-month", start.AddDate(0, 0, 3), -25000)
	br2sugPutExpense(t, app, "outside-month", start.AddDate(0, -1, 3), -270000)
	statuses := []budgeting.Status{{Budget: budget}}
	window := period.NewWindow(period.Month, start, time.Monday)
	got, issues := budgetSummaryActualSpend(app, statuses, window, false, currency.Rates{Base: "USD"}, "USD")
	if got != 25000 || len(issues) != 0 {
		t.Fatalf("actual monthly spend = %d, issues %v; want 25000 and none", got, issues)
	}
}

func TestBV6FFutureHeadlineSpendIsZero(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	budget := domain.Budget{ID: "monthly", CategoryID: "transport-cat", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")}
	br2sugPutExpense(t, app, "future-window", start.AddDate(0, 0, 3), -5000)
	got, issues := budgetSummaryActualSpend(app, []budgeting.Status{{Budget: budget}}, period.NewWindow(period.Month, start, time.Monday), false, currency.Rates{Base: "USD"}, "USD")
	if got != 0 || len(issues) != 0 {
		t.Fatalf("future actual spend = %d, issues %v; want zero and none", got, issues)
	}
}

func TestBV6FSuggestionSuppressesRaiseWhileUnderPace(t *testing.T) {
	app := br2sugTestApp(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	budget := domain.Budget{ID: "under-pace", Name: "Transportation", CategoryID: "transport-cat", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(100000, "USD")}
	if err := app.PutBudget(budget); err != nil {
		t.Fatal(err)
	}
	for i, amount := range []int64{130000, 90000, 80000} {
		date := start.AddDate(0, -(i + 1), 10)
		br2sugPutExpense(t, app, "under-pace-history-"+date.Format("2006-01"), date, -amount)
	}
	if recommendation, _, ok := budgetRecommendationAt(app, budget, start); ok && recommendation.Direction == budgeting.SuggestionRaise {
		t.Fatal("a raise appeared while the current period was under pace and only one of three prior periods overspent")
	}
}

func TestBV6FLowerSuggestionNeverDropsBelowOutstandingBills(t *testing.T) {
	suggested, err := budgetSuggestionBillFloor(money.New(12600, "USD"), money.New(110000, "USD"), "USD", currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if suggested.Amount != 110000 {
		t.Fatalf("lower suggestion = %d, want committed bill floor 110000", suggested.Amount)
	}
}

func TestBV6FRecurringLineNamesDueAndPaidBills(t *testing.T) {
	cases := []struct {
		name   string
		status budgeting.Status
		want   string
	}{
		{name: "due", status: budgeting.Status{Budget: domain.Budget{ID: "due"}, Spent: money.New(0, "USD"), Remaining: money.New(130000, "USD"),
			Committed:          budgeting.CommittedResult{Required: money.New(110000, "USD")},
			FreeAfterCommitted: money.New(20000, "USD"), Projection: budgeting.Pace{Committed: money.New(110000, "USD")}}, want: "$200.00 free after $1,100.00 in committed bills"},
		{name: "paid", status: budgeting.Status{Budget: domain.Budget{ID: "paid"}, Spent: money.New(110000, "USD"), Remaining: money.New(20000, "USD"), FreeAfterCommitted: money.New(110000, "USD"),
			Committed:  budgeting.CommittedResult{Required: money.Zero("USD")},
			Projection: budgeting.Pace{Committed: money.New(110000, "USD")}}, want: "$200.00 free; $1,100.00 in bills already paid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(budgetRecurringStatusLine(tc.status, "$200.00"))
			line := byTestID(f, "span", "budget-free-after-committed-"+tc.status.Budget.ID)
			if line == nil || line.Text() != tc.want {
				t.Fatalf("recurring line = %v, want %q", line, tc.want)
			}
		})
	}
}

func TestBV6FYearlyPaceCopyNamesTheBudgetPeriod(t *testing.T) {
	status := budgeting.Status{Budget: domain.Budget{ID: "yearly", Period: domain.PeriodYearly},
		Spent: money.New(25000, "USD"), Projection: budgeting.Pace{Elapsed: .79}}
	result := budgetRowPaceNotes(false, budgetRowProps{Status: status, PaceMarkerPct: 79}, status, prefs.Prefs{})
	f := render.New(t)
	f.Render(result.paceLegend)
	legend := byTestID(f, "span", "budget-pace-legend-yearly")
	if legend == nil || legend.Text() != "Today's pace: 79% through this budget period" {
		t.Fatalf("yearly pace legend = %v, want a period-neutral caption", legend)
	}
}

func TestBV6FPaceCaptionIsHiddenWhenProjectionWarningIsShown(t *testing.T) {
	status := budgeting.Status{Budget: domain.Budget{ID: "pace"},
		Projection: budgeting.Pace{OverBy: money.New(20800, "USD")}}
	props := budgetRowProps{Status: status, PaceCaption: "Running $122.00 hot", PaceHot: true, PaceOver: "$208.00"}
	result := budgetRowPaceNotes(false, props, status, prefs.Prefs{})
	f := render.New(t)
	f.Render(result.paceMarkLine)
	if f.Text() != "" {
		t.Fatalf("duplicate pace caption = %q", f.Text())
	}
}
