// SPDX-License-Identifier: MIT

package payoff

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildPlanCheckedRejectsOverflowWithoutInventingPayoff(t *testing.T) {
	for _, debts := range [][]Debt{
		{{Balance: math.MaxInt64/2 + 1, MinPayment: 100}, {Balance: math.MaxInt64/2 + 1, MinPayment: 100}},
		{{Balance: 100, MinPayment: math.MaxInt64}, {Balance: 100, MinPayment: 1}},
		{{Balance: math.MaxInt64 - 1, AprPercent: 12, MinPayment: 100}},
		{{Balance: 100, AprPercent: math.MaxFloat64, MinPayment: 100}},
	} {
		plan, ok, err := BuildPlanChecked(debts, 0, Avalanche)
		if ok || !errors.Is(err, money.ErrOverflow) || plan.Months != 0 || len(plan.Schedule) != 0 {
			t.Fatalf("overflow invented a payoff: %+v / %t / %v", plan, ok, err)
		}
	}
}

func TestBuildPlanCheckedPreservesNormalPlanAndNonviability(t *testing.T) {
	debts := []Debt{{Name: "Card", Balance: 50000, AprPercent: 12, MinPayment: 5000}, {Name: "Loan", Balance: 100000, AprPercent: 4, MinPayment: 10000}}
	got, ok, err := BuildPlanChecked(debts, 1000, Avalanche)
	legacy, legacyOK := BuildPlan(debts, 1000, Avalanche)
	if err != nil || !ok || !legacyOK || !reflect.DeepEqual(got, legacy) || got.Months != 10 {
		t.Fatalf("normal plan changed: %+v / %t / %v", got, ok, err)
	}
	if _, ok, err := BuildPlanChecked([]Debt{{Balance: 100000, AprPercent: 24, MinPayment: 1}}, 0, Avalanche); ok || err != nil {
		t.Fatalf("nonviability became calculation failure: %t / %v", ok, err)
	}
}
