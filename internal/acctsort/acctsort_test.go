// SPDX-License-Identifier: MIT

package acctsort

import (
	"sort"
	"testing"
)

func TestRiskFirstLess(t *testing.T) {
	tests := []struct {
		name           string
		staleI, staleJ bool
		balI, balJ     int64
		want           bool
	}{
		{"stale leads healthy", true, false, 100, 900, true},
		{"healthy trails stale", false, true, 900, 100, false},
		{"both stale, bigger balance first", true, true, 500, 100, true},
		{"both stale, smaller balance later", true, true, 100, 500, false},
		{"both healthy, bigger balance first", false, false, 900, 100, true},
		{"both healthy, equal balance keeps order", false, false, 100, 100, false},
		{"liability (negative) trails asset when both healthy", false, false, -50, 10, false},
	}
	for _, tc := range tests {
		if got := RiskFirstLess(tc.staleI, tc.staleJ, tc.balI, tc.balJ); got != tc.want {
			t.Errorf("%s: RiskFirstLess(%v,%v,%d,%d) = %v, want %v",
				tc.name, tc.staleI, tc.staleJ, tc.balI, tc.balJ, got, tc.want)
		}
	}
}

// TestRiskFirstSortStable checks the comparator drives a full stable sort so every
// stale account leads every healthy one, and balances order within each tier.
func TestRiskFirstSortStable(t *testing.T) {
	type acct struct {
		id    string
		stale bool
		bal   int64
	}
	in := []acct{
		{"healthy-big", false, 1000},
		{"stale-small", true, 50},
		{"healthy-small", false, 20},
		{"stale-big", true, 800},
	}
	sort.SliceStable(in, func(i, j int) bool {
		return RiskFirstLess(in[i].stale, in[j].stale, in[i].bal, in[j].bal)
	})
	want := []string{"stale-big", "stale-small", "healthy-big", "healthy-small"}
	for i, w := range want {
		if in[i].id != w {
			t.Errorf("position %d = %q, want %q", i, in[i].id, w)
		}
	}
}
