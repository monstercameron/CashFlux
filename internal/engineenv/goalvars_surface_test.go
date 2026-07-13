// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddGoalVarsSurface(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	goals := []domain.Goal{
		{ID: "g1", Name: "Emergency Fund", TargetAmount: usd(1000000), CurrentAmount: usd(250000)},
		{ID: "g2", Name: "Vacation", VarName: "trip", TargetAmount: usd(300000), CurrentAmount: usd(300000)},
	}
	vars := Vars(Data{Goals: goals, Rates: currency.Rates{Base: "USD"}, Now: now})

	want := map[string]float64{
		"goal_emergency_fund_target":    10000,
		"goal_emergency_fund_saved":     2500,
		"goal_emergency_fund_remaining": 7500,
		"goal_emergency_fund_percent":   25,
		"goal_trip_target":              3000, // explicit VarName "trip"
		"goal_trip_saved":               3000,
		"goal_trip_remaining":           0,
		"goal_trip_percent":             100,
	}
	for name, exp := range want {
		if got, ok := vars[name]; !ok {
			t.Errorf("missing surface var %q", name)
		} else if got != exp {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
	if _, ok := vars["goal_vacation_target"]; ok {
		t.Error("explicit VarName should override the name-derived slug")
	}
}

func TestAddGoalVarsKindAware(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	linked := func(id, goalID string, s domain.TaskStatus) domain.Task {
		return domain.Task{ID: id, Title: id, Status: s, Priority: domain.PriorityMedium,
			RelatedType: domain.RelatedGoal, RelatedID: goalID}
	}
	goals := []domain.Goal{
		// checklist: 1 of 2 to-dos done → 50% progress
		{ID: "c1", Name: "Plan Wedding", Kind: domain.GoalKindChecklist},
		// milestone: marked done → 100% progress, done=1
		{ID: "m1", Name: "Renew Passport", Kind: domain.GoalKindMilestone, DoneAt: now},
		// habit: 3 of 6 weekly check-ins, last two consecutive → streak
		{ID: "h1", Name: "Weekly Review", Kind: domain.GoalKindHabit, HabitCadence: domain.CadenceWeekly, HabitTarget: 6,
			CheckIns: []time.Time{now, now.AddDate(0, 0, -7), now.AddDate(0, 0, -14)}},
		// financial with a linked to-do (to-dos count for all kinds; progress stays money-based)
		{ID: "f1", Name: "New Car", Kind: domain.GoalKindFinancial, TargetAmount: usd(1000000), CurrentAmount: usd(500000)},
	}
	tasks := []domain.Task{
		linked("t1", "c1", domain.StatusDone),
		linked("t2", "c1", domain.StatusOpen),
		linked("t3", "f1", domain.StatusOpen),
	}
	vars := Vars(Data{Goals: goals, Tasks: tasks, Rates: currency.Rates{Base: "USD"}, Now: now})

	want := map[string]float64{
		"goal_plan_wedding_progress":     50,
		"goal_plan_wedding_tasks_done":   1,
		"goal_plan_wedding_tasks_total":  2,
		"goal_plan_wedding_done":         0,
		"goal_renew_passport_progress":   100,
		"goal_renew_passport_done":       1,
		"goal_weekly_review_progress":    50, // 3 of 6
		"goal_weekly_review_streak":      3,
		"goal_weekly_review_tasks_total": 0,  // no linked to-dos
		"goal_new_car_progress":          50, // money-based
		"goal_new_car_tasks_total":       1,  // financial goal can still have linked to-dos
		"goal_new_car_streak":            0,
	}
	for name, exp := range want {
		if got, ok := vars[name]; !ok {
			t.Errorf("missing surface var %q", name)
		} else if got != exp {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
}

func TestEarmarkSurface(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accounts := []domain.Account{
		{ID: "acc-check", Name: "Checking", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD"},
	}
	txns := []domain.Transaction{
		{ID: "t1", AccountID: "acc-check", Amount: usd(100000), Date: now.AddDate(0, 0, -5)}, // $1,000 balance
	}
	// Trip: target $2,000, saved $500, plus a $300 earmark against Checking.
	goals := []domain.Goal{
		{ID: "g1", Name: "Trip", TargetAmount: usd(200000), CurrentAmount: usd(50000),
			Allocations: []domain.GoalAllocation{{AccountID: "acc-check", Amount: usd(30000)}}},
	}
	vars := Vars(Data{Accounts: accounts, Transactions: txns, Goals: goals,
		Rates: currency.Rates{Base: "USD"}, Now: now})

	want := map[string]float64{
		"goal_trip_earmarked":        300,
		"goal_trip_coverage":         800, // saved 500 + earmarked 300
		"goal_trip_covered_pct":      40,  // 800 / 2000 * 100
		"earmarked_total":            300,
		"account_checking_balance":   1000,
		"account_checking_earmarked": 300,
		"account_checking_free":      700, // 1000 − 300
		"liquid_cash":                1000,
		"unreserved_cash":            700, // liquid_cash − earmarked_total
	}
	for name, exp := range want {
		if got, ok := vars[name]; !ok {
			t.Errorf("missing surface var %q", name)
		} else if got != exp {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
}

func TestGoalVarBasesCollision(t *testing.T) {
	goals := []domain.Goal{{ID: "g1", Name: "House"}, {ID: "g2", Name: "House"}}
	bases := GoalVarBases(goals)
	if len(bases) != 2 || bases[0].Prefix != "goal_house_" || bases[1].Prefix != "goal_house_2_" {
		t.Errorf("collision handling wrong: %+v", bases)
	}
}
