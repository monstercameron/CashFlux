// SPDX-License-Identifier: MIT

package payoff

import "testing"

func TestBuildPlanSchedule(t *testing.T) {
	debts := []Debt{
		{Name: "Card", Balance: 200000, AprPercent: 20, MinPayment: 5000},
		{Name: "Store", Balance: 50000, AprPercent: 25, MinPayment: 2000},
	}
	plan, ok := BuildPlan(debts, 30000, Snowball)
	if !ok {
		t.Fatal("BuildPlan not viable")
	}
	if len(plan.Schedule) != plan.Months {
		t.Fatalf("schedule length = %d, want Months %d", len(plan.Schedule), plan.Months)
	}
	if plan.Schedule[len(plan.Schedule)-1] != 0 {
		t.Errorf("last scheduled balance = %d, want 0 (debt-free)", plan.Schedule[len(plan.Schedule)-1])
	}
	// The remaining balance only falls (a viable plan makes progress every month).
	prev := int64(1 << 62)
	for i, b := range plan.Schedule {
		if b < 0 {
			t.Errorf("schedule[%d] = %d, must be >= 0", i, b)
		}
		if b > prev {
			t.Errorf("schedule[%d] = %d rose above the previous %d — burn-down must be non-increasing", i, b, prev)
		}
		prev = b
	}
}

func TestBuildPlanScheduleSingleDebt(t *testing.T) {
	plan, ok := BuildPlan([]Debt{{Name: "X", Balance: 10000, AprPercent: 0, MinPayment: 0}}, 4000, Snowball)
	if !ok {
		t.Fatal("not viable")
	}
	// 10000 at $40/mo, no interest -> 6000, 2000, 0 over 3 months.
	want := []int64{6000, 2000, 0}
	if len(plan.Schedule) != len(want) {
		t.Fatalf("schedule = %v, want %v", plan.Schedule, want)
	}
	for i := range want {
		if plan.Schedule[i] != want[i] {
			t.Errorf("schedule[%d] = %d, want %d", i, plan.Schedule[i], want[i])
		}
	}
}
