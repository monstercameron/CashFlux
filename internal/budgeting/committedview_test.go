// SPDX-License-Identifier: MIT

package budgeting

import "testing"

func TestCommittedPercentages(t *testing.T) {
	cases := []struct {
		name                  string
		limit, spent, commit  int64
		wantSpent, wantCommit int
	}{
		{name: "separate segments", limit: 10000, spent: 2500, commit: 1500, wantSpent: 25, wantCommit: 15},
		{name: "clips to remaining cap", limit: 10000, spent: 7500, commit: 4000, wantSpent: 75, wantCommit: 25},
		{name: "over spent fills the limit", limit: 10000, spent: 12000, commit: 1000, wantSpent: 100, wantCommit: 0},
		{name: "nonpositive limit", limit: 0, spent: 100, commit: 100, wantSpent: 0, wantCommit: 0},
		{name: "negative commitment", limit: 10000, spent: 1000, commit: -1, wantSpent: 10, wantCommit: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spent, committed := CommittedPercentages(tc.limit, tc.spent, tc.commit)
			if spent != tc.wantSpent || committed != tc.wantCommit {
				t.Errorf("percentages = (%d, %d), want (%d, %d)", spent, committed, tc.wantSpent, tc.wantCommit)
			}
		})
	}
}

func TestComparePriorSpend(t *testing.T) {
	cases := []struct {
		name                  string
		spent, limit          int64
		wantPercent, wantFill int
		wantDelta             int64
	}{
		{name: "under limit", spent: 2500, limit: 10000, wantPercent: 25, wantFill: 25, wantDelta: 7500},
		{name: "over limit", spent: 12500, limit: 10000, wantPercent: 125, wantFill: 100, wantDelta: -2500},
		{name: "no limit", spent: 500, limit: 0, wantDelta: -500},
		{name: "negative spend", spent: -100, limit: 10000, wantDelta: 10100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComparePriorSpend(tc.spent, tc.limit)
			if got.Percent != tc.wantPercent || got.Fill != tc.wantFill || got.Delta != tc.wantDelta {
				t.Errorf("comparison = %+v, want Percent=%d Fill=%d Delta=%d", got, tc.wantPercent, tc.wantFill, tc.wantDelta)
			}
		})
	}
}
