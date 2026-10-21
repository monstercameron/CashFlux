// SPDX-License-Identifier: MIT

package recurringview

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildMonthlyFactsConvertBeforeCheckedAggregation(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	noDueWindow := now.AddDate(0, 1, 0)
	flows := []domain.Recurring{
		{ID: "income", Amount: money.New(10000, "USD"), Cadence: domain.CadenceMonthly, NextDue: noDueWindow},
		{ID: "yearly-euro", Amount: money.New(-12000, "EUR"), Cadence: domain.CadenceYearly, NextDue: noDueWindow},
		{ID: "paused", Amount: money.New(2000, "USD"), Cadence: domain.CadenceMonthly, NextDue: noDueWindow, Paused: true},
	}
	model := Build(flows, now, "USD", currency.Rates{
		Base: "USD", Rates: map[string]float64{"EUR": 1.5},
	})
	if !model.MonthlyAvailable || model.MonthlyIn != 12000 || model.MonthlyOut != 1500 || model.MonthlyNet != 10500 {
		t.Fatalf("monthly summary = %+v", model)
	}
	if len(model.Flows) != 3 || model.Flows[1].Recurring.ID != "yearly-euro" || model.Flows[1].MonthlyAmount != -1500 {
		t.Fatalf("base converted flow facts = %+v", model.Flows)
	}
	if got := model.Flows[2]; !got.MonthlyAvailable || got.MonthlyAmount != 2000 {
		t.Fatalf("paused flow should remain in monthly roster totals: %+v", got)
	}
	if len(model.Occurrences) != 0 {
		t.Fatalf("all monthly flows are due after the next-30-day window: %+v", model.Occurrences)
	}
}

func TestBuildKeepsKnownRowsWhenFXForOneFlowIsUnavailable(t *testing.T) {
	model := Build([]domain.Recurring{
		{ID: "known", Amount: money.New(-2500, "USD"), Cadence: domain.CadenceMonthly},
		{ID: "unknown-fx", Amount: money.New(100, "EUR"), Cadence: domain.CadenceMonthly},
	}, time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC), "USD", currency.Rates{Base: "USD"})
	if model.MonthlyAvailable || model.MonthlyError == nil {
		t.Fatalf("partial aggregate appeared complete: %+v", model)
	}
	if !model.Flows[0].MonthlyAvailable || model.Flows[0].MonthlyAmount != -2500 {
		t.Fatalf("known row was hidden by unrelated FX failure: %+v", model.Flows[0])
	}
	if model.Flows[1].MonthlyAvailable || model.Flows[1].MonthlyError == nil {
		t.Fatalf("missing FX was presented as a value: %+v", model.Flows[1])
	}
	if model.IncomeShareAvailable || !model.ExpenseShareAvailable || !model.Flows[0].ShareAvailable || model.Flows[0].SharePercent != 100 {
		t.Fatalf("unrelated expense share should remain independently known: %+v", model)
	}
}

func TestBuildMonthlyCadenceAndAggregateOverflowStayUnavailable(t *testing.T) {
	weekly := Build([]domain.Recurring{{ID: "weekly-overflow", Amount: money.New(math.MaxInt64, "USD"), Cadence: domain.CadenceWeekly}}, time.Now(), "USD", currency.Rates{Base: "USD"})
	if weekly.Flows[0].MonthlyAvailable || weekly.Flows[0].MonthlyError == nil || weekly.MonthlyAvailable {
		t.Fatalf("weekly multiplication overflow was hidden: %+v", weekly)
	}

	model := Build([]domain.Recurring{
		{ID: "max", Amount: money.New(math.MaxInt64, "USD"), Cadence: domain.CadenceMonthly},
		{ID: "one", Amount: money.New(1, "USD"), Cadence: domain.CadenceMonthly},
	}, time.Now(), "USD", currency.Rates{Base: "USD"})
	if !model.Flows[0].MonthlyAvailable || !model.Flows[1].MonthlyAvailable || model.MonthlyAvailable || model.MonthlyError == nil {
		t.Fatalf("aggregate overflow corrupted row-level facts: %+v", model)
	}
}

func TestBuildUpcomingWindowPausedAndCapsOverdueOccurrences(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	flows := []domain.Recurring{
		{ID: "overdue", Amount: money.New(-100, "USD"), Cadence: domain.CadenceWeekly, NextDue: now.AddDate(0, 0, -70)},
		{ID: "cutoff", Amount: money.New(-200, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)},
		{ID: "paused", Amount: money.New(300, "USD"), Cadence: domain.CadenceDaily, NextDue: now, Paused: true},
		{ID: "after-cutoff", Amount: money.New(400, "USD"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 0, 31)},
	}
	model := Build(flows, now, "USD", currency.Rates{Base: "USD"})
	if len(model.Occurrences) != 9 {
		t.Fatalf("occurrence count = %d, want 8 overdue plus cutoff occurrence", len(model.Occurrences))
	}
	for i := 0; i < 8; i++ {
		if model.Occurrences[i].Recurring.ID != "overdue" || !model.Occurrences[i].Overdue {
			t.Fatalf("overdue order/cap mismatch at %d: %+v", i, model.Occurrences[i])
		}
	}
	last := model.Occurrences[8]
	if last.Recurring.ID != "cutoff" || last.Overdue || !last.AmountAvailable || last.Amount.Amount != -200 {
		t.Fatalf("cutoff occurrence missing or malformed: %+v", last)
	}
	if model.UpcomingIn != 0 || model.UpcomingOut != 1000 || !model.UpcomingTotalsAvailable {
		t.Fatalf("upcoming totals = %+v", model)
	}
}

func TestBuildUpcomingFXAndMinIntMagnitudeAreUnavailable(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	model := Build([]domain.Recurring{
		{ID: "unknown", Amount: money.New(-100, "EUR"), Cadence: domain.CadenceMonthly, NextDue: now},
		{ID: "min", Amount: money.New(math.MinInt64, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
	}, now, "USD", currency.Rates{Base: "USD"})
	if model.UpcomingTotalsAvailable || model.UpcomingTotalsError == nil {
		t.Fatalf("unavailable occurrences looked complete: %+v", model)
	}
	if model.Occurrences[0].AmountAvailable {
		t.Fatalf("missing FX occurrence got a plausible amount: %+v", model.Occurrences[0])
	}
	if !model.Occurrences[1].AmountAvailable || !strings.Contains(model.UpcomingTotalsError.Error(), "overflow") {
		t.Fatalf("MinInt64 magnitude error not surfaced: %+v", model)
	}
}

func TestBuildDoesNotMutateInputOrReorderTiedSourceRows(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	flows := []domain.Recurring{
		{ID: "second", Amount: money.New(20, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
		{ID: "first", Amount: money.New(10, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
	}
	before := append([]domain.Recurring(nil), flows...)
	model := Build(flows, now, "USD", currency.Rates{Base: "USD"})
	if flows[0] != before[0] || flows[1] != before[1] {
		t.Fatal("Build mutated caller's schedule slice")
	}
	if model.Flows[0].Recurring.ID != "second" || model.Flows[1].Recurring.ID != "first" {
		t.Fatalf("stable tie order changed: %+v", model.Flows)
	}
}

func TestMonthlyExpenseTotalAndAnnualizeAreChecked(t *testing.T) {
	total, err := MonthlyExpenseTotal([]FlowFact{
		{Recurring: domain.Recurring{ID: "bill"}, MonthlyAmount: -125, MonthlyAvailable: true},
		{Recurring: domain.Recurring{ID: "income"}, MonthlyAmount: 500, MonthlyAvailable: true},
	})
	if err != nil || total != 125 {
		t.Fatalf("monthly expense total = %d, %v", total, err)
	}
	if _, err := MonthlyExpenseTotal([]FlowFact{{Recurring: domain.Recurring{ID: "unknown"}}}); err == nil {
		t.Fatal("unavailable fact did not invalidate selected subtotal")
	}
	if _, err := AnnualizeMonthly(math.MaxInt64); err == nil {
		t.Fatal("annualization overflow was hidden")
	}
}
