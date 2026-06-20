package payoff

import (
	"testing"
	"time"
)

func TestDebtFreeMonth(t *testing.T) {
	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		months int
		want   string // "YYYY-MM"
	}{
		{"one month clears this month", 1, "2026-06"},
		{"twelve months", 12, "2027-05"},
		{"nothing owed returns the start month", 0, "2026-06"},
		{"crosses the year boundary", 8, "2027-01"},
		{"long mortgage", 170, "2040-07"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DebtFreeMonth(start, tc.months).Format("2006-01")
			if got != tc.want {
				t.Errorf("DebtFreeMonth(%d) = %s, want %s", tc.months, got, tc.want)
			}
		})
	}
}

func TestBuildPlanClearedMonths(t *testing.T) {
	debts := []Debt{
		{Name: "Card", Balance: 200000, AprPercent: 20, MinPayment: 5000},
		{Name: "Store", Balance: 50000, AprPercent: 25, MinPayment: 2000},
	}
	plan, ok := BuildPlan(debts, 30000, Snowball)
	if !ok {
		t.Fatal("BuildPlan not viable")
	}
	if len(plan.ClearedMonths) != len(plan.Order) {
		t.Fatalf("ClearedMonths (%d) must be parallel to Order (%d)", len(plan.ClearedMonths), len(plan.Order))
	}
	// Months are 1-based, non-decreasing, and the last equals the plan length.
	prev := 0
	for i, m := range plan.ClearedMonths {
		if m < 1 || m < prev {
			t.Errorf("ClearedMonths[%d] = %d (prev %d) — must be >=1 and non-decreasing", i, m, prev)
		}
		prev = m
	}
	if plan.ClearedMonths[len(plan.ClearedMonths)-1] != plan.Months {
		t.Errorf("last debt cleared at month %d, want plan length %d", plan.ClearedMonths[len(plan.ClearedMonths)-1], plan.Months)
	}
}
