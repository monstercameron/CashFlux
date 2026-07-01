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

func TestGoalVarBasesCollision(t *testing.T) {
	goals := []domain.Goal{{ID: "g1", Name: "House"}, {ID: "g2", Name: "House"}}
	bases := GoalVarBases(goals)
	if len(bases) != 2 || bases[0].Prefix != "goal_house_" || bases[1].Prefix != "goal_house_2_" {
		t.Errorf("collision handling wrong: %+v", bases)
	}
}
