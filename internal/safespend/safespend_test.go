// SPDX-License-Identifier: MIT

package safespend

import "testing"

func TestCompute(t *testing.T) {
	cases := []struct {
		name                          string
		liquid, bills, goals, budgets int64
		wantSafe                      int64
		wantNeg                       bool
	}{
		{"positive", 200000, 50000, 30000, 40000, 80000, false}, // 2000 − 500 − 300 − 400 = 800
		{"exactly zero", 100000, 40000, 30000, 30000, 0, false}, // nets to 0 → not negative
		{"negative / overcommitted", 100000, 80000, 30000, 20000, -30000, true},
		{"no commitments", 150000, 0, 0, 0, 150000, false},
		{"overdrawn liquid stays negative", -5000, 0, 0, 0, -5000, true},
		{"negative commitments clamped to 0", 100000, -50000, -1, -1, 100000, false},
	}
	for _, c := range cases {
		b := Compute(c.liquid, c.bills, c.goals, c.budgets, "USD")
		if b.SafeToSpend != c.wantSafe {
			t.Errorf("%s: SafeToSpend = %d, want %d", c.name, b.SafeToSpend, c.wantSafe)
		}
		if b.IsNegative != c.wantNeg {
			t.Errorf("%s: IsNegative = %v, want %v", c.name, b.IsNegative, c.wantNeg)
		}
		if b.Currency != "USD" {
			t.Errorf("%s: Currency not carried through", c.name)
		}
	}
}

func TestComputeClampsBuckets(t *testing.T) {
	b := Compute(100000, -10, -20, -30, "USD")
	if b.BillsDue != 0 || b.GoalContributions != 0 || b.CommittedBudgets != 0 {
		t.Errorf("negative buckets should clamp to 0, got %+v", b)
	}
}

func TestComputeCategory(t *testing.T) {
	cases := []struct {
		name                   string
		remaining              int64
		daysLeft, daysInPeriod int
		want                   int64
	}{
		{"half the period left", 30000, 15, 30, 15000}, // 300 paced over half → 150
		{"full period left", 30000, 30, 30, 30000},
		{"one day left", 30000, 1, 30, 1000}, // floor(30000*1/30)
		{"floors fractional", 100, 1, 3, 33}, // floor(100/3)
		{"zero days in period guards", 30000, 5, 0, 0},
		{"zero days left", 30000, 0, 30, 0},
		{"negative remaining (over) → 0", -5000, 10, 30, 0},
		{"daysLeft clamped to period", 30000, 99, 30, 30000},
	}
	for _, c := range cases {
		if got := ComputeCategory(c.remaining, c.daysLeft, c.daysInPeriod); got != c.want {
			t.Errorf("%s: ComputeCategory(%d,%d,%d) = %d, want %d", c.name, c.remaining, c.daysLeft, c.daysInPeriod, got, c.want)
		}
	}
}
