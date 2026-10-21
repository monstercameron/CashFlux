// SPDX-License-Identifier: MIT

package safespend

import (
	"math"
	"testing"
)

func TestComputeCheckedMatchesComputeAndClampsCommitments(t *testing.T) {
	for _, tc := range []struct {
		liquid, bills, goals, budgets int64
	}{
		{1000, 200, 300, 100},
		{100, -20, 30, -5},
		{-100, 0, 20, 0},
	} {
		want := Compute(tc.liquid, tc.bills, tc.goals, tc.budgets, "USD")
		got, err := ComputeChecked(tc.liquid, tc.bills, tc.goals, tc.budgets, "USD")
		if err != nil || got != want {
			t.Fatalf("ComputeChecked(%+v) = %+v, %v; want %+v", tc, got, err, want)
		}
	}
}

func TestComputeCheckedReportsUnderflow(t *testing.T) {
	if got, err := ComputeChecked(math.MinInt64, 1, 0, 0, "USD"); err == nil || got != (Breakdown{}) {
		t.Fatalf("underflow result = %+v, %v", got, err)
	}
}
