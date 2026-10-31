// SPDX-License-Identifier: MIT

package budgetplan

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestProjectWithGoalRatesUsesConvertedRemainingWithoutMutation(t *testing.T) {
	goal := domain.Goal{ID: "goal", CategoryID: "food", TargetAmount: usd(1000), CurrentAmount: usd(100), TargetDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Allocations: []domain.GoalAllocation{{AccountID: "foreign", Amount: money.New(400, "EUR")}}}
	before := goal
	before.Allocations = append([]domain.GoalAllocation(nil), goal.Allocations...)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	projection := ProjectWithGoalRates(nil, []domain.Goal{goal}, 2026, 10, "USD", rates)
	if projection.Error != nil || projection.GoalsByCategory["food"][10] != 100 || projection.GoalsByCategory["food"][11] != 0 {
		t.Fatalf("rate-aware remaining should require $1 in November: %+v", projection)
	}
	if !reflect.DeepEqual(goal, before) {
		t.Fatal("projection normalized and rewrote raw goal money")
	}
	goal.Allocations[0].Amount = money.New(450, "EUR")
	complete := ProjectWithGoalRates(nil, []domain.Goal{goal}, 2026, 10, "USD", rates)
	if complete.Error != nil || sumMonths(complete.GoalsByCategory["food"]) != 0 {
		t.Fatalf("covered goal still asks for derived contributions: %+v", complete)
	}
}

func TestProjectWithGoalRatesReportsUnknownCoverageInsteadOfZero(t *testing.T) {
	goal := domain.Goal{ID: "unknown", CategoryID: "food", TargetAmount: usd(1000), CurrentAmount: usd(100), TargetDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Allocations: []domain.GoalAllocation{{Amount: money.New(2000, "EUR")}}}
	projection := ProjectWithGoalRates(nil, []domain.Goal{goal}, 2026, 10, "USD", usdRates)
	if projection.Error == nil {
		t.Fatal("unknown goal funding became a known zero monthly projection")
	}
	if _, err := projection.PerBudget([]string{"food"}, map[string]map[string]bool{"food": {"food": true}}); err == nil {
		t.Fatal("partial projection was exposed as a complete budget forecast")
	}
	legacy := Project(nil, []domain.Goal{goal}, 2026, 10, "USD", usdRates)
	if legacy.Error != nil {
		t.Fatal("legacy caller's omission contract changed")
	}
}

func TestProjectWithGoalRatesPreservesKnownExplicitContributions(t *testing.T) {
	goal := domain.Goal{ID: "explicit", CategoryID: "food", TargetAmount: usd(1000), CurrentAmount: usd(100), MonthlyContribution: money.New(100, "EUR"),
		Allocations: []domain.GoalAllocation{{Amount: money.New(2000, "JPY")}}}
	projection := ProjectWithGoalRates(nil, []domain.Goal{goal}, 2026, 10, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if projection.Error != nil || projection.GoalsByCategory["food"][10] != 200 || projection.GoalsByCategory["food"][11] != 200 {
		t.Fatalf("known explicit contributions incorrectly depended on unknown coverage: %+v", projection)
	}
	if got := ProjectWithGoalRates(nil, []domain.Goal{goal}, 2026, 10, "USD", usdRates); got.Error == nil {
		t.Fatal("unconvertible explicit contribution silently became zero")
	}
}
