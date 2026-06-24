// SPDX-License-Identifier: MIT

package insights

import (
	"testing"
)

func TestAffordAnswer(t *testing.T) {
	tests := []struct {
		name        string
		q           AffordQuery
		balance     int64
		monthlyNet  int64
		reservePct  float64
		wantCan     bool
		wantSurplus int64 // Available - Amount
	}{
		{
			name:    "affordable now from balance",
			q:       AffordQuery{Amount: 50000, MonthsAhead: 0},
			balance: 200000, monthlyNet: 0, reservePct: 0,
			wantCan:     true,
			wantSurplus: 150000, // 200000 - 50000
		},
		{
			name:    "affordable after saving 3 months",
			q:       AffordQuery{Amount: 150000, MonthsAhead: 3},
			balance: 0, monthlyNet: 60000, reservePct: 0,
			wantCan:     true,
			wantSurplus: 30000, // (0+60000*3) - 150000
		},
		{
			name:    "shortfall with reserve",
			q:       AffordQuery{Amount: 100000, MonthsAhead: 1},
			balance: 100000, monthlyNet: 10000, reservePct: 0.20,
			// projected = 100000+10000 = 110000, reserve = 100000*0.20 = 20000
			// available = 110000-20000 = 90000 < 100000 → not affordable
			wantCan:     false,
			wantSurplus: -10000,
		},
		{
			name: "zero balance zero net",
			q:    AffordQuery{Amount: 10000, MonthsAhead: 1},
			// projected = 0, available = 0, shortfall = 10000
			wantCan:     false,
			wantSurplus: -10000,
		},
		{
			name:    "months defaults to 1 when zero in query",
			q:       AffordQuery{Amount: 50000, MonthsAhead: 0},
			balance: 0, monthlyNet: 60000, reservePct: 0,
			// defaultMonthsAhead=1: projected = 60000 > 50000
			wantCan:     true,
			wantSurplus: 10000,
		},
		{
			name:    "assumptions include target label",
			q:       AffordQuery{Amount: 50000, MonthsAhead: 6, TargetLabel: "December"},
			balance: 200000, monthlyNet: 10000, reservePct: 0,
			wantCan:     true,
			wantSurplus: 210000, // 200000+6*10000-50000
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AffordAnswer(tc.q, tc.balance, tc.monthlyNet, tc.reservePct)
			if got.CanAfford != tc.wantCan {
				t.Errorf("CanAfford = %v, want %v (Available=%d Amount=%d)", got.CanAfford, tc.wantCan, got.Available, tc.q.Amount)
			}
			if got.Surplus != tc.wantSurplus {
				t.Errorf("Surplus = %d, want %d", got.Surplus, tc.wantSurplus)
			}
			if len(got.Assumptions) == 0 {
				t.Error("Assumptions should not be empty")
			}
		})
	}
}

func TestAffordAnswerAssumptionCount(t *testing.T) {
	// Without reserve, without target label: 3 assumptions.
	q := AffordQuery{Amount: 10000, MonthsAhead: 2}
	r := AffordAnswer(q, 0, 5000, 0)
	if len(r.Assumptions) != 3 {
		t.Errorf("expected 3 assumptions, got %d: %v", len(r.Assumptions), r.Assumptions)
	}

	// With reserve and target label: 5 assumptions.
	q2 := AffordQuery{Amount: 10000, MonthsAhead: 2, TargetLabel: "June"}
	r2 := AffordAnswer(q2, 100000, 5000, 0.10)
	if len(r2.Assumptions) != 5 {
		t.Errorf("expected 5 assumptions, got %d: %v", len(r2.Assumptions), r2.Assumptions)
	}
}

func TestFmtMinorUnits(t *testing.T) {
	tests := []struct {
		v    int64
		want string
	}{
		{0, "$0.00"},
		{100, "$1.00"},
		{120000, "$1,200.00"},
		{50000, "$500.00"},
		{-50000, "-$500.00"},
		{1000000, "$10,000.00"},
		{100000000, "$1,000,000.00"},
	}
	for _, tc := range tests {
		if got := fmtMinorUnits(tc.v); got != tc.want {
			t.Errorf("fmtMinorUnits(%d) = %q, want %q", tc.v, got, tc.want)
		}
	}
}
