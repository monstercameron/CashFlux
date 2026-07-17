// SPDX-License-Identifier: MIT

package milestones

import (
	"reflect"
	"testing"
)

func TestHighestRungBelow(t *testing.T) {
	cases := []struct {
		nw   int64
		want int64
	}{
		{0, 0},
		{999_999, 0},         // just under $10k
		{1_000_000, 1_000_000}, // exactly $10k
		{3_000_000, 2_500_000}, // $30k → $25k rung
		{10_000_000, 10_000_000},
		{123_400_000, 100_000_000}, // $1.234M → $1M rung
		{999_000_000, 500_000_000}, // between $5M ladder top — clamps to $5M
	}
	for _, c := range cases {
		if got := highestRungBelow(c.nw); got != c.want {
			t.Errorf("highestRungBelow(%d) = %d, want %d", c.nw, got, c.want)
		}
	}
}

func TestDetect(t *testing.T) {
	in := Input{
		ReachedGoals:  []string{"Emergency fund", ""},
		NetWorthMinor: 12_000_000, // $120k → $100k rung
		NoSpendDays:   5,
		KeptBudgets:   3,
		KeptPeriodKey: "2026-06",
	}
	got := Detect(in)
	want := []Milestone{
		{Key: "goal:Emergency fund", Kind: KindGoalReached, Name: "Emergency fund"},
		{Key: "networth:10000000", Kind: KindNetWorth, Value: 10_000_000},
		{Key: "nospend:5", Kind: KindNoSpendStreak, Value: 5},
		{Key: "kept:2026-06", Kind: KindKeptBudgets, Value: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Detect() = %+v, want %+v", got, want)
	}
}

func TestDetectNothingToCelebrate(t *testing.T) {
	in := Input{
		NetWorthMinor: 500_000, // under the first rung
		NoSpendDays:   2,       // under the streak floor
		KeptBudgets:   0,
	}
	if got := Detect(in); len(got) != 0 {
		t.Errorf("Detect() = %+v, want empty", got)
	}
}

func TestDetectKeptBudgetsNeedsPeriodKey(t *testing.T) {
	// A kept-budget count with no period key can't be deduped, so it's skipped.
	in := Input{KeptBudgets: 2, KeptPeriodKey: ""}
	if got := Detect(in); len(got) != 0 {
		t.Errorf("Detect() = %+v, want empty (no period key)", got)
	}
}
