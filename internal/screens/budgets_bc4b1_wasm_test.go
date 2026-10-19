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
)

func TestBudgetSummaryPlanUsesStandingLimit(t *testing.T) {
	status := budgeting.Status{
		Budget: domain.Budget{ID: "weekly", Period: domain.PeriodWeekly, Limit: money.New(6000, "USD")},
		Limit:  money.New(11000, "USD"),
	}
	budgets := budgetSummarySpendingBudgets([]budgeting.Status{status})
	if len(budgets) != 1 || budgets[0].Limit.Amount != 6000 {
		t.Fatalf("summary plan budgets = %+v; want stored standing limit 6000", budgets)
	}
}

func TestBudgetSpreadRequiresCompleteNormalization(t *testing.T) {
	good := budgetView{Statuses: []budgeting.Status{{Budget: domain.Budget{ID: "ok"}}}}
	if !budgetSpreadReady(good) {
		t.Fatal("complete view should permit spread")
	}
	failed := good
	failed.Statuses = []budgeting.Status{{Budget: domain.Budget{ID: "fx-gap"}, Error: &budgeting.BudgetEvaluationError{BudgetID: "fx-gap"}}}
	if budgetSpreadReady(failed) {
		t.Fatal("failed budget must block a spread based on incomplete totals")
	}
	failed = good
	failed.Incomplete = []string{"incomplete"}
	if budgetSpreadReady(failed) {
		t.Fatal("incomplete rollup must block spread")
	}
	failed = good
	failed.BannerIncome = 10
	failed.RolledOver = 5
	failed.TotalLimit = 2
	failed.SavingsAssigned = 1
	if amount, ok := budgetSpreadAmount(failed); !ok || amount != 12 {
		t.Fatalf("complete spread amount = %d, %t, want 12, true", amount, ok)
	}
	failed.BannerIncome = int64(^uint64(0) >> 1)
	failed.RolledOver = 1
	if _, ok := budgetSpreadAmount(failed); ok {
		t.Fatal("overflowing pool must block spread")
	}
	failed = good
	failed.BannerIncome = int64(^uint64(0) >> 1)
	failed.TotalLimit = -1
	if _, ok := budgetSpreadAmount(failed); ok {
		t.Fatal("overflowing leftover subtraction must block spread")
	}
}

func TestBudgetEffectiveCapsSkipsFailedStatus(t *testing.T) {
	v := budgetView{Statuses: []budgeting.Status{
		{Budget: domain.Budget{ID: "failed"}, Limit: money.New(100, "USD"), Error: &budgeting.BudgetEvaluationError{BudgetID: "failed"}},
		{Budget: domain.Budget{ID: "good"}, Limit: money.New(250, "USD")},
	}}
	got := budgetEffectiveCaps(v)
	if _, ok := got["failed"]; ok {
		t.Fatalf("failed budget received a guessed cap: %v", got)
	}
	if got["good"] != 250 {
		t.Fatalf("good cap = %d, want 250", got["good"])
	}
}

func TestBudgetMinorToMajorIsIntegerExact(t *testing.T) {
	for _, tc := range []struct {
		minor int64
		base  string
		want  string
	}{{1234567, "USD", "12345.67"}, {1200, "USD", "12"}, {12345, "JPY", "12345"}} {
		if got := minorToMajorStr(tc.minor, tc.base); got != tc.want {
			t.Errorf("minorToMajorStr(%d, %s) = %q, want %q", tc.minor, tc.base, got, tc.want)
		}
	}
}

func TestBudgetLastMonthHeadlineUsesPriorSpendAndMonthlyPlan(t *testing.T) {
	spent, plan := budgetHeadlineAmounts(budgetView{
		LastMonthMode: true, TotalSpent: 900, TotalLimit: 1200,
		LastTotalSpent: 300, LastTotalLimit: 500,
	}, 1400)
	if spent != 900 || plan != 1400 {
		t.Fatalf("last-month headline = %d/%d, want selected-window spend and monthly plan 900/1400", spent, plan)
	}
}

func TestBudgetAmountSortDoesNotCompareCurrenciesAsMinorUnits(t *testing.T) {
	statuses := []budgeting.Status{
		{Budget: domain.Budget{ID: "usd"}, Limit: money.New(900, "USD")},
		{Budget: domain.Budget{ID: "eur"}, Limit: money.New(100000, "EUR")},
	}
	sortBudgetStatuses(statuses, "amount", nil)
	if statuses[0].Limit.Currency != "EUR" || statuses[1].Limit.Currency != "USD" {
		t.Fatalf("mixed currency sort compared raw minor units: %+v", statuses)
	}
}

func TestSavingsMonthlyBaseCheckedReportsMissingFX(t *testing.T) {
	if _, err := savingsMonthlyBaseChecked(50000, "EUR", "USD", currency.Rates{Base: "USD"}); err == nil {
		t.Fatal("missing FX must be reported so the household total can be marked incomplete")
	}
}

func TestBudgetNowNormalizesWallClockBeforePeriodBounds(t *testing.T) {
	pacific := time.FixedZone("Pacific", -7*60*60)
	localNow := time.Date(2026, time.October, 31, 18, 0, 0, 0, pacific)
	want := time.Date(2026, time.October, 31, 18, 0, 0, 0, time.UTC)
	if got := budgetNowAt(localNow); !got.Equal(want) {
		t.Fatalf("budget wall clock = %s, want %s", got, want)
	}
}

func TestBudgetRestPerDayUsesBudgetCurrency(t *testing.T) {
	base := "USD"
	b := domain.Budget{ID: "eur", Limit: money.New(10000, "EUR")}
	builder := newBudgetViewBuilder(budgeting.ViewInput{Base: base}, false)
	builder.addBudget(budgeting.ViewBudget{
		Valid: true, Budget: b,
		Status:     budgeting.Status{Budget: b, Remaining: money.New(5000, "EUR")},
		RestPerDay: 1250,
	})
	if got, want := builder.view.ProratedRest[b.ID], fmtMoney(money.New(1250, "EUR")); got != want {
		t.Fatalf("per-day figure = %q, want %q", got, want)
	}
}

func TestSinkingFundSetAsideConvertsToBaseCurrency(t *testing.T) {
	goal := domain.Goal{
		IsSinkingFund: true, TargetAmount: money.New(1000, "EUR"), CurrentAmount: money.Zero("EUR"),
		TargetDate: time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	got, err := sinkingFundSetAsideBase(goal, time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil || got != 668 {
		t.Fatalf("base set-aside = %d, %v; want 668 USD minor units", got, err)
	}
}

func TestBudgetAdjustOverlayOverflowIsOmitted(t *testing.T) {
	b := domain.Budget{ID: "overflow", Limit: money.New(-1, "USD")}
	status := budgeting.Status{Budget: b, Limit: money.New(int64(^uint64(0)>>1), "USD")}
	got := budgetAdjustOverlays(budgetView{Statuses: []budgeting.Status{status}}, []domain.Budget{b}, nil)
	if _, ok := got[b.ID]; ok {
		t.Fatalf("overflowing overlay was guessed: %v", got)
	}
}
