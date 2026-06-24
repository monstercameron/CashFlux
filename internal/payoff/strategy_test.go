// SPDX-License-Identifier: MIT

package payoff

import (
	"reflect"
	"testing"
)

// origPrincipal sums the starting balances owed.
func origPrincipal(debts []Debt) int64 {
	var sum int64
	for _, d := range debts {
		if d.Balance > 0 {
			sum += d.Balance
		}
	}
	return sum
}

func TestBuildPlanStrategiesDiffer(t *testing.T) {
	// Card: high balance, high APR. Store: low balance, low APR — so snowball
	// (smallest first) and avalanche (highest APR first) pick different focuses.
	debts := []Debt{
		{Name: "Card", Balance: 100000, AprPercent: 24, MinPayment: 3000},
		{Name: "Store", Balance: 20000, AprPercent: 10, MinPayment: 2000},
	}

	snow, ok := BuildPlan(debts, 20000, Snowball)
	if !ok {
		t.Fatal("snowball not viable")
	}
	if want := []string{"Store", "Card"}; !reflect.DeepEqual(snow.Order, want) {
		t.Errorf("snowball order = %v, want %v", snow.Order, want)
	}

	aval, ok := BuildPlan(debts, 20000, Avalanche)
	if !ok {
		t.Fatal("avalanche not viable")
	}
	if want := []string{"Card", "Store"}; !reflect.DeepEqual(aval.Order, want) {
		t.Errorf("avalanche order = %v, want %v", aval.Order, want)
	}

	// Avalanche pays no more interest than snowball (it's the interest-optimal order).
	if aval.TotalInterest > snow.TotalInterest {
		t.Errorf("avalanche interest %d > snowball %d", aval.TotalInterest, snow.TotalInterest)
	}

	// Conservation: everything paid is principal + interest, for both strategies.
	orig := origPrincipal(debts)
	for _, p := range []Plan{snow, aval} {
		if p.TotalPaid != orig+p.TotalInterest {
			t.Errorf("TotalPaid %d != principal %d + interest %d", p.TotalPaid, orig, p.TotalInterest)
		}
		if p.Months <= 0 {
			t.Errorf("Months = %d, want positive", p.Months)
		}
	}
}

func TestBuildPlanNoInterest(t *testing.T) {
	// One 0% debt, $120k, $10k/month → exactly 12 months, no interest.
	debts := []Debt{{Name: "X", Balance: 120000, AprPercent: 0, MinPayment: 0}}
	p, ok := BuildPlan(debts, 10000, Snowball)
	if !ok {
		t.Fatal("should be viable")
	}
	if p.Months != 12 || p.TotalInterest != 0 || p.TotalPaid != 120000 {
		t.Errorf("plan = %+v, want 12 months / 0 interest / 120000 paid", p)
	}
	if want := []string{"X"}; !reflect.DeepEqual(p.Order, want) {
		t.Errorf("order = %v, want [X]", p.Order)
	}
}

func TestBuildPlanNoDebts(t *testing.T) {
	p, ok := BuildPlan(nil, 1000, Snowball)
	if !ok {
		t.Fatal("no debts should be viable")
	}
	if p.Months != 0 || len(p.Order) != 0 {
		t.Errorf("plan = %+v, want empty", p)
	}
}

func TestBuildPlanSkipsAlreadyPaid(t *testing.T) {
	debts := []Debt{
		{Name: "Paid", Balance: 0, AprPercent: 10, MinPayment: 1000},
		{Name: "Owed", Balance: 50000, AprPercent: 0, MinPayment: 5000},
	}
	p, ok := BuildPlan(debts, 5000, Snowball)
	if !ok {
		t.Fatal("should be viable")
	}
	// "Paid" was never owed, so it isn't in the payoff order.
	if want := []string{"Owed"}; !reflect.DeepEqual(p.Order, want) {
		t.Errorf("order = %v, want [Owed]", p.Order)
	}
}

func TestBuildPlanNotViable(t *testing.T) {
	// Crushing APR, tiny payment, no extra → interest always outpaces payment.
	debts := []Debt{{Name: "Trap", Balance: 100000, AprPercent: 100, MinPayment: 100}}
	if _, ok := BuildPlan(debts, 0, Snowball); ok {
		t.Error("expected not viable")
	}
}

func TestBuildPlanNegativeExtra(t *testing.T) {
	debts := []Debt{{Name: "X", Balance: 1000, AprPercent: 0, MinPayment: 100}}
	if _, ok := BuildPlan(debts, -1, Snowball); ok {
		t.Error("negative extra should be rejected")
	}
}

func TestBuildPlanZeroBudget(t *testing.T) {
	// No minimums and no extra → no firepower.
	debts := []Debt{{Name: "X", Balance: 1000, AprPercent: 0, MinPayment: 0}}
	if _, ok := BuildPlan(debts, 0, Snowball); ok {
		t.Error("zero budget should be rejected")
	}
}
