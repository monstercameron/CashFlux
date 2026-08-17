// SPDX-License-Identifier: MIT

package spendmix

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var rates = currency.Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "EUR": 1.1}}

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func month() (time.Time, time.Time) { return d(2026, time.March, 1), d(2026, time.April, 1) }

func cat(id string, class domain.CategoryClass) domain.Category {
	return domain.Category{ID: id, Name: id, CategoryClass: class}
}

func spend(catID string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{CategoryID: catID, Date: when, Amount: money.New(-minor, "USD")}
}

func sampleCats() []domain.Category {
	return []domain.Category{
		cat("rent", domain.ClassFixed),
		cat("insurance", domain.ClassNonMonthly),
		cat("dining", domain.ClassFlex),
		cat("misc", ""), // empty class reads as flex
	}
}

func TestSummarizeSplitsByHowMuchChoiceWasInvolved(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("rent", d(2026, time.March, 1), 180_000),
		spend("insurance", d(2026, time.March, 5), 40_000),
		spend("dining", d(2026, time.March, 10), 30_000),
		spend("misc", d(2026, time.March, 12), 10_000),
	}
	m := Summarize(txns, sampleCats(), from, to, rates)
	if m.FixedMinor != 180_000 {
		t.Errorf("fixed = %d, want 180000", m.FixedMinor)
	}
	if m.NonMonthlyMinor != 40_000 {
		t.Errorf("non-monthly = %d, want 40000", m.NonMonthlyMinor)
	}
	// An empty class reads as flex, so misc joins dining.
	if m.FlexMinor != 40_000 {
		t.Errorf("flex = %d, want 40000", m.FlexMinor)
	}
	if m.TotalMinor != 260_000 {
		t.Errorf("total = %d, want 260000", m.TotalMinor)
	}
	if m.CommittedMinor() != 220_000 {
		t.Errorf("committed = %d, want 220000", m.CommittedMinor())
	}
}

func TestNonMonthlyIsItsOwnBucket(t *testing.T) {
	// Neither a monthly commitment nor a free choice. Folding it into fixed
	// overstates how trapped the month was; folding it into flex suggests it could
	// simply be skipped.
	from, to := month()
	m := Summarize([]domain.Transaction{spend("insurance", d(2026, time.March, 5), 40_000)},
		sampleCats(), from, to, rates)
	if m.FixedMinor != 0 || m.FlexMinor != 0 {
		t.Errorf("non-monthly leaked into another bucket: %+v", m)
	}
	if m.NonMonthlyMinor != 40_000 {
		t.Errorf("non-monthly = %d, want 40000", m.NonMonthlyMinor)
	}
}

func TestUncategorizedIsNotCalledDiscretionary(t *testing.T) {
	// The flattering guess: it tells the reader they had more room than they did.
	from, to := month()
	txns := []domain.Transaction{
		spend("", d(2026, time.March, 3), 50_000),
		spend("dining", d(2026, time.March, 4), 10_000),
	}
	m := Summarize(txns, sampleCats(), from, to, rates)
	if m.FlexMinor != 10_000 {
		t.Errorf("flex = %d, want only the categorized 10000", m.FlexMinor)
	}
	if m.UncategorizedMinor != 50_000 {
		t.Errorf("uncategorized = %d, want 50000", m.UncategorizedMinor)
	}
	if m.TotalMinor != 60_000 {
		t.Errorf("total = %d — uncategorized still counts toward the month", m.TotalMinor)
	}
}

func TestIncomeTransfersAndExclusionsAreNotSpending(t *testing.T) {
	from, to := month()
	income := domain.Transaction{CategoryID: "dining", Date: d(2026, time.March, 2), Amount: money.New(50_000, "USD")}
	transfer := spend("dining", d(2026, time.March, 3), 20_000)
	transfer.TransferAccountID = "savings"
	excluded := spend("dining", d(2026, time.March, 4), 30_000)
	excluded.ExcludeFromReports = true
	m := Summarize([]domain.Transaction{income, transfer, excluded}, sampleCats(), from, to, rates)
	if m.TotalMinor != 0 {
		t.Errorf("total = %d, want 0", m.TotalMinor)
	}
}

func TestOutOfPeriodSpendingIsIgnored(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("dining", d(2026, time.February, 28), 10_000),
		spend("dining", d(2026, time.March, 1), 20_000),
		spend("dining", d(2026, time.April, 1), 40_000),
	}
	if m := Summarize(txns, sampleCats(), from, to, rates); m.TotalMinor != 20_000 {
		t.Errorf("total = %d, want only the 20000 inside the month", m.TotalMinor)
	}
}

func TestForeignSpendConvertsToBase(t *testing.T) {
	from, to := month()
	tx := domain.Transaction{CategoryID: "dining", Date: d(2026, time.March, 5), Amount: money.New(-1_000, "EUR")}
	if m := Summarize([]domain.Transaction{tx}, sampleCats(), from, to, rates); m.FlexMinor != 1_100 {
		t.Errorf("flex = %d, want 1100", m.FlexMinor)
	}
}

func TestFlexShareRefusesAMonthThatDidNotHappen(t *testing.T) {
	var m Mix
	if _, ok := m.FlexSharePct(); ok {
		t.Error("a period with no spending must not report a share")
	}
	m = Mix{FlexMinor: 25_000, TotalMinor: 100_000}
	got, ok := m.FlexSharePct()
	if !ok || got != 25 {
		t.Errorf("share = %v (ok=%v), want 25", got, ok)
	}
}

func TestCompareSortsWorstOverspendFirst(t *testing.T) {
	r := Compare(
		map[string]int64{"b1": 40_000, "b2": 20_000, "b3": 10_000},
		map[string]int64{"b1": 41_000, "b2": 35_000, "b3": 4_000},
		map[string]string{"b1": "Groceries", "b2": "Dining", "b3": "Travel"},
		map[string]string{},
	)
	if len(r.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(r.Rows))
	}
	if r.Rows[0].Label != "Dining" {
		t.Errorf("first row = %q, want the biggest overspend (Dining)", r.Rows[0].Label)
	}
	if r.Rows[2].Label != "Travel" {
		t.Errorf("last row = %q, want the biggest underspend (Travel)", r.Rows[2].Label)
	}
}

func TestOverAndUnderAreNotNetted(t *testing.T) {
	// $300 over on one and $300 under on another is not a month that went to
	// plan, and a net zero would say it was.
	r := Compare(
		map[string]int64{"a": 100_000, "b": 100_000},
		map[string]int64{"a": 130_000, "b": 70_000},
		map[string]string{}, map[string]string{},
	)
	if r.OverMinor != 30_000 || r.UnderMinor != 30_000 {
		t.Errorf("over/under = %d/%d, want 30000/30000", r.OverMinor, r.UnderMinor)
	}
	if r.NetMinor() != 0 {
		t.Errorf("net = %d, want 0 — and this is exactly why it is not the headline", r.NetMinor())
	}
}

func TestABudgetWithNoLimitIsSkippedNotReportedAsOnPlan(t *testing.T) {
	r := Compare(
		map[string]int64{"a": 0, "b": 10_000},
		map[string]int64{"a": 50_000, "b": 10_000},
		map[string]string{}, map[string]string{},
	)
	if len(r.Rows) != 1 || r.Rows[0].BudgetID != "b" {
		t.Errorf("rows = %+v, want only the budget that has a limit", r.Rows)
	}
	if r.PlannedMinor != 10_000 || r.ActualMinor != 10_000 {
		t.Errorf("totals = %d/%d — the limitless budget must not contribute", r.PlannedMinor, r.ActualMinor)
	}
}

func TestOnPlanToleratesTheNearMissesNobodyActsOn(t *testing.T) {
	// A budget hit to the dollar does not happen, and flagging a $2 overspend on
	// $400 trains people to ignore the flag.
	r := Compare(
		map[string]int64{"a": 40_000, "b": 40_000},
		map[string]int64{"a": 40_200, "b": 48_000},
		map[string]string{}, map[string]string{},
	)
	byID := map[string]Variance{}
	for _, v := range r.Rows {
		byID[v.BudgetID] = v
	}
	if !byID["a"].OnPlan() {
		t.Errorf("a $2 overspend on $400 (%.2f%%) should read as on plan", byID["a"].PctOfLimit)
	}
	if byID["b"].OnPlan() {
		t.Errorf("a 20%% overspend (%.2f%%) is not on plan", byID["b"].PctOfLimit)
	}
	// On plan and over are independent: "a" is technically over and still fine.
	if !byID["a"].Over() {
		t.Error("a $2 overspend is still over, even while on plan")
	}
}

func TestTheSameInputAlwaysOrdersTheSameWay(t *testing.T) {
	// Map iteration is random; two budgets equally far from plan must still come
	// out in a stable order or the report reshuffles on every render.
	limits := map[string]int64{"z": 10_000, "a": 10_000}
	spends := map[string]int64{"z": 12_000, "a": 12_000}
	for i := range 5 {
		r := Compare(limits, spends, map[string]string{}, map[string]string{})
		if r.Rows[0].BudgetID != "a" {
			t.Fatalf("run %d put %q first, want a stable \"a\"", i, r.Rows[0].BudgetID)
		}
	}
}
