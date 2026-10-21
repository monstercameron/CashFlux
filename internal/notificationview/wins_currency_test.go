// SPDX-License-Identifier: MIT

package notificationview

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/milestones"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareWinsUsesRateAwareGoalCompletionAndPreservesMilestoneKeys(t *testing.T) {
	name := "Reserve · 家 / USD"
	allocationGoal := domain.Goal{ID: "g", Name: name, TargetAmount: money.New(10000, "USD"), CurrentAmount: money.New(0, "USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "euro", Amount: money.New(10000, "EUR")}}}
	savedGoal := allocationGoal
	savedGoal.Allocations = nil
	savedGoal.CurrentAmount = money.New(10000, "EUR")
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		goal  domain.Goal
		rates map[string]float64
		want  bool
	}{
		{"foreign allocation below target", allocationGoal, map[string]float64{"EUR": .5}, false},
		{"foreign allocation covers target", allocationGoal, map[string]float64{"EUR": 2}, true},
		{"unknown allocation rate", allocationGoal, nil, false},
		{"invalid allocation rate", allocationGoal, map[string]float64{"EUR": math.NaN()}, false},
		{"foreign saved below target", savedGoal, map[string]float64{"EUR": .5}, false},
		{"foreign saved covers target", savedGoal, map[string]float64{"EUR": 2}, true},
		{"unknown saved rate", savedGoal, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rates := currency.Rates{Base: "USD", Rates: tc.rates}
			got := PrepareWins(WinsInput{Goals: []domain.Goal{tc.goal}, Rates: rates, Now: now})
			if (len(got.ReachedGoals) == 1) != tc.want {
				t.Fatalf("reached goals %v, want celebrated=%t", got.ReachedGoals, tc.want)
			}
			canonical := goals.EvaluateGoal(tc.goal, goals.TaskSummary{}, rates, now)
			if (len(got.ReachedGoals) == 1) != (canonical.Err == nil && canonical.Complete) {
				t.Fatalf("milestone completion diverged from canonical evaluation: %+v", canonical)
			}
			wins := milestones.Detect(got)
			if tc.want {
				if got.ReachedGoals[0] != name || len(wins) != 1 || wins[0].Kind != milestones.KindGoalReached || wins[0].Key != "goal:"+name || wins[0].Name != name {
					t.Fatalf("goal name or persisted milestone key changed: %+v", wins)
				}
			} else if len(wins) != 0 {
				t.Fatalf("unreached/unknown goal produced a milestone: %+v", wins)
			}
			if tc.goal.Name != name || tc.goal.TargetAmount != money.New(10000, "USD") {
				t.Fatal("milestone preparation normalized the persisted goal")
			}
		})
	}
}

func TestPrepareWinsReevaluatesGoalWhenOnlyRatesChange(t *testing.T) {
	goal := domain.Goal{ID: "g", Name: "Travel", TargetAmount: money.New(10000, "USD"), CurrentAmount: money.New(0, "USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "euro", Amount: money.New(10000, "EUR")}}}
	in := WinsInput{Goals: []domain.Goal{goal}, Rates: currency.Rates{Base: "USD"}, Now: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)}
	for _, tc := range []struct {
		rates map[string]float64
		want  bool
	}{{map[string]float64{"EUR": .5}, false}, {map[string]float64{"EUR": 2}, true}, {nil, false}} {
		in.Rates.Rates = tc.rates
		if got := PrepareWins(in); (len(got.ReachedGoals) == 1) != tc.want {
			t.Fatalf("rate table %v produced reached goals %v, want celebrated=%t", tc.rates, got.ReachedGoals, tc.want)
		}
	}
}
