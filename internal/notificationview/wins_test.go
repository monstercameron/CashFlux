// SPDX-License-Identifier: MIT

package notificationview

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/networthview"
)

func TestPrepareWinsGoalKindsAndSeparateRawNettedLedgers(t *testing.T) {
	now := time.Date(2028, 3, 1, 8, 0, 0, 0, time.UTC)
	in := WinsInput{
		Now: now, Rates: currency.Rates{Base: "USD"}, NoSpendLimit: 90, WeekStart: time.Monday,
		Accounts:     []domain.Account{{ID: "a", Currency: "USD", Class: domain.ClassAsset, OpeningBalance: money.New(2000000, "USD")}},
		Transactions: []domain.Transaction{{ID: "raw", AccountID: "a", Date: now.AddDate(0, 0, -1), CategoryID: "food", Amount: money.New(-200, "USD")}},
		// Budget refund netting reduced spend below the limit; raw spend would exceed it.
		BudgetLedger: budgeting.NettedLedger{{ID: "net", Date: now.AddDate(0, 0, -1), CategoryID: "food", Amount: money.New(-50, "USD")}},
		Budgets:      []domain.Budget{{CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}},
		Goals: []domain.Goal{
			{ID: "financial", Name: "Funded", TargetAmount: money.New(100, "USD"), CurrentAmount: money.New(100, "USD")},
			{ID: "checklist", Name: "Checklist", Kind: domain.GoalKindChecklist},
			{ID: "habit", Name: "Habit", Kind: domain.GoalKindHabit, HabitTarget: 1, CheckIns: []time.Time{now}},
			{ID: "milestone", Name: "Milestone", Kind: domain.GoalKindMilestone, DoneAt: now},
			{ID: "empty-checklist", Name: "Empty", Kind: domain.GoalKindChecklist},
		},
		Tasks: []domain.Task{{ID: "step", Status: domain.StatusDone, RelatedType: domain.RelatedGoal, RelatedID: "checklist"}},
	}
	got := PrepareWins(in)
	if !reflect.DeepEqual(got.ReachedGoals, []string{"Funded", "Checklist", "Habit", "Milestone"}) {
		t.Fatalf("reached goals %v", got.ReachedGoals)
	}
	if got.NetWorthMinor != 1999800 || got.NoSpendDays != 0 || got.KeptBudgets != 1 || got.KeptPeriodKey != "2028-02" || got.Now != now {
		t.Fatalf("milestone input %+v", got)
	}
	if in.Transactions[0].Amount.Amount != -200 || in.BudgetLedger[0].Amount.Amount != -50 || in.Goals[0].Name != "Funded" {
		t.Fatal("wins preparation changed its inputs")
	}
	precomputed := networthview.Snapshot{NetWorthResult: ledger.NetWorthResult{Net: money.New(123, "USD")}, Known: true}
	in.NetWorth = &precomputed
	if got := PrepareWins(in); got.NetWorthMinor != 123 {
		t.Fatal("provided net-worth snapshot was not reused")
	}
}

func TestPrepareWinsMonthlyBudgetEligibilityAndPerBudgetErrors(t *testing.T) {
	now := time.Date(2026, 1, 31, 9, 0, 0, 0, time.FixedZone("local", -5*3600))
	base := domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}
	for _, tc := range []struct {
		name   string
		budget domain.Budget
		amount int64
		when   time.Time
		want   int
	}{
		{"at limit", base, -100, now.AddDate(0, -1, 0), 1},
		{"no spend", base, 0, now.AddDate(0, -1, 0), 0},
		{"overspent", base, -101, now.AddDate(0, -1, 0), 0},
		{"current month excluded", base, -50, now, 0},
		{"before previous month excluded", base, -50, now.AddDate(0, -2, -1), 0},
		{"weekly excluded", domain.Budget{CategoryID: "food", Period: domain.PeriodWeekly, Limit: money.New(100, "USD")}, -50, now.AddDate(0, -1, 0), 0},
		{"blank limit currency inherits base", domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.Money{Amount: 100}}, -50, now.AddDate(0, -1, 0), 1},
		{"missing conversion skipped", domain.Budget{CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(100, "GBP")}, -50, now.AddDate(0, -1, 0), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PrepareWins(WinsInput{Now: now, Rates: currency.Rates{Base: "USD"}, Budgets: []domain.Budget{tc.budget},
				BudgetLedger: budgeting.NettedLedger{{Date: tc.when, CategoryID: "food", Amount: money.New(tc.amount, "USD")}}})
			if got.KeptBudgets != tc.want || got.KeptPeriodKey != "2025-12" {
				t.Fatalf("previous month %s kept %d, want %d", got.KeptPeriodKey, got.KeptBudgets, tc.want)
			}
		})
	}
	bad := base
	bad.Limit = money.New(100, "GBP")
	got := PrepareWins(WinsInput{Now: now, Rates: currency.Rates{Base: "USD"}, Budgets: []domain.Budget{bad, base},
		BudgetLedger: budgeting.NettedLedger{{Date: now.AddDate(0, -1, 0), CategoryID: "food", Amount: money.New(-50, "USD")}}})
	if got.KeptBudgets != 1 {
		t.Fatal("one conversion error removed another budget's win")
	}
}
