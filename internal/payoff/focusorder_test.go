// SPDX-License-Identifier: MIT

package payoff

import (
	"reflect"
	"testing"
)

func TestFocusOrder(t *testing.T) {
	// A realistic mixed portfolio: two low-rate car loans, a mid-rate personal loan,
	// and two high-rate cards. The clear order (small debts first on their minimums)
	// would NOT match either strategy — this is exactly the case that made the
	// avalanche ladder look unsorted.
	debts := []Debt{
		{Name: "Regression Loan", Balance: 900000, AprPercent: 8.50},
		{Name: "Priya Car", Balance: 2120000, AprPercent: 6.90},
		{Name: "Marcus Car", Balance: 2684000, AprPercent: 7.40},
		{Name: "Rewards Card", Balance: 300000, AprPercent: 24.99},
		{Name: "Travel Card", Balance: 53500, AprPercent: 19.90},
	}

	// Avalanche: strictly highest APR first.
	gotA := FocusOrder(debts, Avalanche)
	wantA := []string{"Rewards Card", "Travel Card", "Regression Loan", "Marcus Car", "Priya Car"}
	if !reflect.DeepEqual(gotA, wantA) {
		t.Errorf("avalanche order = %v, want %v", gotA, wantA)
	}

	// Snowball: strictly smallest balance first.
	gotS := FocusOrder(debts, Snowball)
	wantS := []string{"Travel Card", "Rewards Card", "Regression Loan", "Priya Car", "Marcus Car"}
	if !reflect.DeepEqual(gotS, wantS) {
		t.Errorf("snowball order = %v, want %v", gotS, wantS)
	}
}

func TestFocusOrderTiesAndZeros(t *testing.T) {
	debts := []Debt{
		{Name: "A", Balance: 100000, AprPercent: 20},
		{Name: "B", Balance: 50000, AprPercent: 20}, // same APR, smaller balance
		{Name: "Paid", Balance: 0, AprPercent: 30},  // dropped (nothing owed)
	}
	// Avalanche tie on APR → smaller balance (the quick win) comes first; Paid gone.
	if got := FocusOrder(debts, Avalanche); !reflect.DeepEqual(got, []string{"B", "A"}) {
		t.Errorf("avalanche tie order = %v, want [B A]", got)
	}
	// Snowball → smallest balance first anyway.
	if got := FocusOrder(debts, Snowball); !reflect.DeepEqual(got, []string{"B", "A"}) {
		t.Errorf("snowball order = %v, want [B A]", got)
	}
}
