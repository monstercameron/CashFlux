// SPDX-License-Identifier: MIT

package waterfall

import "testing"

func TestCompute(t *testing.T) {
	tests := []struct {
		name       string
		income     int64
		goals      []GoalQuota
		free       AccountFree
		wantLines  []PlanLine
		wantRemain int64
	}{
		{
			name:   "cascades in priority order until income exhausted",
			income: 2400_00,
			goals: []GoalQuota{
				{GoalID: "emerg", Name: "Emergency", QuotaMinor: 200_00, AccountID: "sav"},
				{GoalID: "vac", Name: "Vacation", QuotaMinor: 150_00, AccountID: "sav"},
				{GoalID: "house", Name: "House", QuotaMinor: 300_00, AccountID: "sav"},
			},
			free: AccountFree{"sav": 5000_00},
			wantLines: []PlanLine{
				{GoalID: "emerg", Name: "Emergency", AccountID: "sav", AmountMinor: 200_00},
				{GoalID: "vac", Name: "Vacation", AccountID: "sav", AmountMinor: 150_00},
				{GoalID: "house", Name: "House", AccountID: "sav", AmountMinor: 300_00},
			},
			wantRemain: 2400_00 - 650_00,
		},
		{
			name:   "income runs out mid-list, last goal partially funded",
			income: 250_00,
			goals: []GoalQuota{
				{GoalID: "a", Name: "A", QuotaMinor: 200_00, AccountID: "x"},
				{GoalID: "b", Name: "B", QuotaMinor: 200_00, AccountID: "x"},
			},
			free: AccountFree{"x": 10000_00},
			wantLines: []PlanLine{
				{GoalID: "a", Name: "A", AccountID: "x", AmountMinor: 200_00},
				{GoalID: "b", Name: "B", AccountID: "x", AmountMinor: 50_00},
			},
			wantRemain: 0,
		},
		{
			name:   "XC7: shared account free balance caps combined plan",
			income: 1000_00,
			goals: []GoalQuota{
				{GoalID: "a", Name: "A", QuotaMinor: 300_00, AccountID: "x"},
				{GoalID: "b", Name: "B", QuotaMinor: 300_00, AccountID: "x"},
			},
			free: AccountFree{"x": 400_00},
			wantLines: []PlanLine{
				{GoalID: "a", Name: "A", AccountID: "x", AmountMinor: 300_00},
				{GoalID: "b", Name: "B", AccountID: "x", AmountMinor: 100_00},
			},
			wantRemain: 1000_00 - 400_00,
		},
		{
			name:   "already-funded goal draws only the remaining quota",
			income: 1000_00,
			goals: []GoalQuota{
				{GoalID: "a", Name: "A", QuotaMinor: 200_00, AlreadyFundedMinor: 150_00, AccountID: "x"},
			},
			free:       AccountFree{"x": 10000_00},
			wantLines:  []PlanLine{{GoalID: "a", Name: "A", AccountID: "x", AmountMinor: 50_00}},
			wantRemain: 950_00,
		},
		{
			name:       "goal without an account is skipped",
			income:     500_00,
			goals:      []GoalQuota{{GoalID: "a", Name: "A", QuotaMinor: 200_00}},
			free:       AccountFree{},
			wantLines:  nil,
			wantRemain: 500_00,
		},
		{
			name:       "zero income yields empty plan",
			income:     0,
			goals:      []GoalQuota{{GoalID: "a", Name: "A", QuotaMinor: 200_00, AccountID: "x"}},
			free:       AccountFree{"x": 1000_00},
			wantLines:  nil,
			wantRemain: 0,
		},
		{
			name:       "fully-funded goal skipped",
			income:     500_00,
			goals:      []GoalQuota{{GoalID: "a", Name: "A", QuotaMinor: 200_00, AlreadyFundedMinor: 200_00, AccountID: "x"}},
			free:       AccountFree{"x": 1000_00},
			wantLines:  nil,
			wantRemain: 500_00,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compute(tc.income, tc.goals, tc.free)
			if len(got.Lines) != len(tc.wantLines) {
				t.Fatalf("got %d lines, want %d: %+v", len(got.Lines), len(tc.wantLines), got.Lines)
			}
			for i, l := range got.Lines {
				if l != tc.wantLines[i] {
					t.Errorf("line %d = %+v, want %+v", i, l, tc.wantLines[i])
				}
			}
			if got.RemainderMinor != tc.wantRemain {
				t.Errorf("remainder = %d, want %d", got.RemainderMinor, tc.wantRemain)
			}
			// Invariant: funded + remainder == income (for positive income).
			if tc.income > 0 && got.FundedMinor+got.RemainderMinor != tc.income {
				t.Errorf("funded %d + remainder %d != income %d", got.FundedMinor, got.RemainderMinor, tc.income)
			}
		})
	}
}
