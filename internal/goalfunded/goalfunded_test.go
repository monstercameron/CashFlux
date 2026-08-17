// SPDX-License-Identifier: MIT

package goalfunded

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

func month() (time.Time, time.Time) { return d(2026, time.August, 1), d(2026, time.September, 1) }

func spend(catID, goalID string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{
		ID: catID + goalID + when.Format("02"), CategoryID: catID, GoalID: goalID,
		Date: when, Amount: money.New(-minor, "USD"),
	}
}

func TestFundedAndOrdinaryAreSeparatedNotDeducted(t *testing.T) {
	// The whole point: goal money still left the account. It is separated, not
	// hidden — a budget screen that quietly shrank its own totals would be a
	// different and worse lie than the one being fixed.
	from, to := month()
	txns := []domain.Transaction{
		spend("cat-home", "", d(2026, time.August, 3), 20_000),
		spend("cat-home", "goal-sofa", d(2026, time.August, 10), 80_000),
	}
	s := Summarize(txns, nil, from, to, rates)
	if s.OrdinaryMinor != 20_000 {
		t.Errorf("ordinary = %d, want 20000", s.OrdinaryMinor)
	}
	if s.FundedMinor != 80_000 {
		t.Errorf("funded = %d, want 80000", s.FundedMinor)
	}
	if s.TotalMinor != 100_000 {
		t.Errorf("total = %d — the money still left the account", s.TotalMinor)
	}
}

// "You would be over, but this came from your holiday fund" is a completely
// different message from "you are over".
func TestAGoalCanRescueABudgetFromReadingAsOverspent(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("cat-home", "", d(2026, time.August, 3), 30_000),
		spend("cat-home", "goal-sofa", d(2026, time.August, 10), 90_000),
	}
	s := Summarize(txns, nil, from, to, rates)
	over, rescued := s.OverBy(50_000)
	if over != 0 {
		t.Errorf("ordinary over = %d, want 0 — only the goal-funded part crossed the limit", over)
	}
	if !rescued {
		t.Error("the total crossed the limit and the ordinary spend did not — that is the sentence worth showing")
	}
}

func TestRealOverspendIsStillReportedAsOverspend(t *testing.T) {
	// A goal must not launder ordinary overspending.
	from, to := month()
	txns := []domain.Transaction{
		spend("cat-home", "", d(2026, time.August, 3), 70_000),
		spend("cat-home", "goal-sofa", d(2026, time.August, 10), 90_000),
	}
	s := Summarize(txns, nil, from, to, rates)
	over, rescued := s.OverBy(50_000)
	if over != 20_000 {
		t.Errorf("ordinary over = %d, want 20000", over)
	}
	if rescued {
		t.Error("ordinary spending was over on its own — nothing rescued it")
	}
}

func TestNoLimitMakesNoOverClaim(t *testing.T) {
	s := Summarize(nil, nil, time.Time{}, time.Time{}, rates)
	if over, rescued := s.OverBy(0); over != 0 || rescued {
		t.Errorf("a budget with no limit reported over=%d rescued=%v", over, rescued)
	}
}

func TestCategoryScopeIsTheCallersBusiness(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("cat-home", "goal-sofa", d(2026, time.August, 5), 50_000),
		spend("cat-food", "goal-sofa", d(2026, time.August, 6), 10_000),
	}
	s := Summarize(txns, map[string]bool{"cat-home": true}, from, to, rates)
	if s.TotalMinor != 50_000 {
		t.Errorf("total = %d, want only the in-scope category", s.TotalMinor)
	}
}

func TestIncomeTransfersAndExcludedRowsDoNotCount(t *testing.T) {
	from, to := month()
	income := domain.Transaction{ID: "i", GoalID: "g", Date: d(2026, time.August, 2), Amount: money.New(50_000, "USD")}
	excluded := spend("cat-home", "g", d(2026, time.August, 3), 50_000)
	excluded.ExcludeFromReports = true
	s := Summarize([]domain.Transaction{income, excluded}, nil, from, to, rates)
	if s.TotalMinor != 0 {
		t.Errorf("total = %d, want 0", s.TotalMinor)
	}
}

func TestGoalsAreNamedSoASurfaceNeedNotSayAGoal(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("cat-home", "goal-sofa", d(2026, time.August, 5), 10_000),
		spend("cat-home", "goal-trip", d(2026, time.August, 6), 10_000),
		spend("cat-home", "goal-sofa", d(2026, time.August, 7), 10_000),
	}
	s := Summarize(txns, nil, from, to, rates)
	if len(s.Goals) != 2 || s.Goals[0] != "goal-sofa" || s.Goals[1] != "goal-trip" {
		t.Errorf("goals = %v, want a sorted, deduplicated [goal-sofa goal-trip]", s.Goals)
	}
}

func TestByGoalTotalsWhatEachGoalPaidFor(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("cat-home", "goal-sofa", d(2026, time.August, 5), 30_000),
		spend("cat-home", "goal-sofa", d(2026, time.August, 6), 20_000),
		spend("cat-home", "goal-trip", d(2026, time.August, 7), 40_000),
	}
	got := ByGoal(txns, from, to, rates)
	if len(got) != 2 {
		t.Fatalf("drawdowns = %d, want 2", len(got))
	}
	// Largest first: the goal that lost the most reads first.
	if got[0].GoalID != "goal-sofa" || got[0].SpentMinor != 50_000 || got[0].Count != 2 {
		t.Errorf("first drawdown = %+v, want goal-sofa 50000 over 2", got[0])
	}
}

func TestByGoalIsStableAcrossRuns(t *testing.T) {
	from, to := month()
	txns := []domain.Transaction{
		spend("c", "goal-b", d(2026, time.August, 5), 10_000),
		spend("c", "goal-a", d(2026, time.August, 6), 10_000),
	}
	for i := range 5 {
		got := ByGoal(txns, from, to, rates)
		if got[0].GoalID != "goal-a" {
			t.Fatalf("run %d put %q first, want a stable goal-a on an equal total", i, got[0].GoalID)
		}
	}
}

// A goal that still shows the full amount after the purchase is a goal lying
// about its own progress.
func TestRemainingReflectsWhatWasSpent(t *testing.T) {
	if got := RemainingMinor(100_000, Drawdown{SpentMinor: 30_000}); got != 70_000 {
		t.Errorf("remaining = %d, want 70000", got)
	}
}

func TestRemainingNeverGoesNegative(t *testing.T) {
	// Spending more than a goal held is real — the difference came from elsewhere
	// — but a negative balance would read as a debt the household does not owe.
	if got := RemainingMinor(100_000, Drawdown{SpentMinor: 160_000}); got != 0 {
		t.Errorf("remaining = %d, want 0 rather than a fictional debt", got)
	}
}

func TestForeignSpendConvertsToBase(t *testing.T) {
	from, to := month()
	tx := domain.Transaction{ID: "e", CategoryID: "c", GoalID: "g",
		Date: d(2026, time.August, 5), Amount: money.New(-1_000, "EUR")}
	s := Summarize([]domain.Transaction{tx}, nil, from, to, rates)
	if s.FundedMinor != 1_100 {
		t.Errorf("funded = %d, want 1100", s.FundedMinor)
	}
}
