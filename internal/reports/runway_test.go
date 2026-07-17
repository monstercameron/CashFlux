// SPDX-License-Identifier: MIT

package reports

import "testing"

func TestEstimateRunway(t *testing.T) {
	tests := []struct {
		name       string
		balance    int64
		burn       int64
		wantMonths int
		wantDays   int
		wantSust   bool
	}{
		{"sustainable when burn zero", 100000, 0, 0, 0, true},
		{"sustainable when income covers (negative burn)", 100000, -500, 0, 0, true},
		{"zero balance with burn", 0, 50000, 0, 0, false},
		{"negative balance with burn", -100, 50000, 0, 0, false},
		{"exact months", 600000, 200000, 3, 0, false},
		{"months plus days", 250000, 100000, 2, 15, false}, // 0.5 month → 15 days
		{"under a month", 50000, 100000, 0, 15, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateRunway(tc.balance, tc.burn)
			if got.Months != tc.wantMonths || got.Days != tc.wantDays || got.Sustainable != tc.wantSust {
				t.Errorf("EstimateRunway(%d,%d) = %+v, want months=%d days=%d sust=%v",
					tc.balance, tc.burn, got, tc.wantMonths, tc.wantDays, tc.wantSust)
			}
		})
	}
}

func TestAverageMonthlyExpense(t *testing.T) {
	tests := []struct {
		name  string
		flows []PeriodFlow
		want  int64
	}{
		{"empty", nil, 0},
		{"all inactive", []PeriodFlow{{}, {}}, 0},
		{
			"averages active buckets",
			[]PeriodFlow{{Income: 5000, Expense: 3000}, {Income: 5000, Expense: 5000}},
			4000,
		},
		{
			"skips fully-empty bucket",
			// Two active months (3000, 5000) averaging 4000; the empty bucket is skipped.
			[]PeriodFlow{{Income: 5000, Expense: 3000}, {}, {Income: 5000, Expense: 5000}},
			4000,
		},
		{
			"income-only month still counts as a zero-spend month",
			[]PeriodFlow{{Income: 5000, Expense: 4000}, {Income: 5000, Expense: 0}},
			2000, // (4000 + 0) / 2
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := AverageMonthlyExpense(tc.flows); got != tc.want {
				t.Errorf("AverageMonthlyExpense = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAverageMonthlyIncome(t *testing.T) {
	tests := []struct {
		name  string
		flows []PeriodFlow
		want  int64
	}{
		{"empty", nil, 0},
		{"all inactive", []PeriodFlow{{}, {}}, 0},
		{
			"averages active buckets, skips empty",
			[]PeriodFlow{{Income: 6000, Expense: 3000}, {}, {Income: 4000, Expense: 5000}},
			5000,
		},
		{
			"expense-only month counts as a zero-income month",
			[]PeriodFlow{{Income: 6000, Expense: 4000}, {Income: 0, Expense: 2000}},
			3000, // (6000 + 0) / 2
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := AverageMonthlyIncome(tc.flows); got != tc.want {
				t.Errorf("AverageMonthlyIncome = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestActiveMonths(t *testing.T) {
	flows := []PeriodFlow{{Income: 1, Expense: 0}, {}, {Income: 0, Expense: 2}, {}}
	if got := ActiveMonths(flows); got != 2 {
		t.Errorf("ActiveMonths = %d, want 2", got)
	}
	if got := ActiveMonths(nil); got != 0 {
		t.Errorf("ActiveMonths(nil) = %d, want 0", got)
	}
}

func TestRunwayFromAverage(t *testing.T) {
	// End-to-end: balance lasts balance/avg months.
	flows := []PeriodFlow{
		{Income: 500000, Expense: 300000},
		{Income: 500000, Expense: 300000},
	}
	avg := AverageMonthlyExpense(flows) // 300000
	r := EstimateRunway(900000, avg)
	if r.Months != 3 || r.Days != 0 {
		t.Errorf("runway = %+v, want 3 months", r)
	}
}
