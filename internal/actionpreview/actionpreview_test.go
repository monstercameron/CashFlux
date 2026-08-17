// SPDX-License-Identifier: MIT

package actionpreview

import "testing"

func TestABetterMoveIsJudgedBetter(t *testing.T) {
	p := Build([]Metric{{Name: "months of runway", Before: 10, After: 12, Goodness: HigherBetter}})
	if len(p.Changed) != 1 {
		t.Fatalf("changed = %d, want 1", len(p.Changed))
	}
	if p.Changed[0].Direction != DirectionBetter {
		t.Errorf("direction = %q, want %q", p.Changed[0].Direction, DirectionBetter)
	}
	if p.Changed[0].DeltaAbs != 2 {
		t.Errorf("delta = %d, want a positive 2", p.Changed[0].DeltaAbs)
	}
}

// A rising balance is good, a rising utilization is not, and no amount of
// looking at the value tells you which you have.
func TestDirectionDependsOnTheMetricNotTheSign(t *testing.T) {
	up := Build([]Metric{{Name: "utilization", Before: 40, After: 55, Goodness: LowerBetter}})
	if up.Changed[0].Direction != DirectionWorse {
		t.Errorf("rising utilization = %q, want %q", up.Changed[0].Direction, DirectionWorse)
	}
	down := Build([]Metric{{Name: "interest paid", Before: 500, After: 300, Goodness: LowerBetter}})
	if down.Changed[0].Direction != DirectionBetter {
		t.Errorf("falling interest = %q, want %q", down.Changed[0].Direction, DirectionBetter)
	}
}

func TestANeutralMetricMovesWithoutAVerdict(t *testing.T) {
	p := Build([]Metric{{Name: "payoff month", Before: 5, After: 9, Goodness: Neutral}})
	if p.Changed[0].Direction != DirectionUnknown {
		t.Errorf("direction = %q, want %q — the app should not guess", p.Changed[0].Direction, DirectionUnknown)
	}
}

// "Goal funding unchanged" is not filler — it answers the question the reader
// was already asking, and its absence is indistinguishable from nobody checking.
func TestUnchangedMetricsAreStatedNotDropped(t *testing.T) {
	p := Build([]Metric{
		{Name: "goal funding", Before: 100, After: 100, Goodness: HigherBetter},
		{Name: "months of runway", Before: 10, After: 12, Goodness: HigherBetter},
	})
	if len(p.Unchanged) != 1 || p.Unchanged[0] != "goal funding" {
		t.Errorf("unchanged = %v, want goal funding named", p.Unchanged)
	}
	if len(p.Changed) != 1 {
		t.Errorf("changed = %d, want 1", len(p.Changed))
	}
}

// A preview that leads with its improvements is an advertisement; one that leads
// with its costs is a decision aid.
func TestWorseThingsComeFirst(t *testing.T) {
	p := Build([]Metric{
		{Name: "runway", Before: 12, After: 14, Goodness: HigherBetter},
		{Name: "cash on hand", Before: 500, After: 300, Goodness: HigherBetter},
		{Name: "payoff month", Before: 1, After: 2, Goodness: Neutral},
	})
	if len(p.Changed) != 3 {
		t.Fatalf("changed = %d, want 3", len(p.Changed))
	}
	if p.Changed[0].Metric.Name != "cash on hand" {
		t.Errorf("first = %q, want the thing that gets worse", p.Changed[0].Metric.Name)
	}
	if p.Changed[2].Metric.Name != "runway" {
		t.Errorf("last = %q, want the improvement", p.Changed[2].Metric.Name)
	}
}

func TestWorsensIsTheFactAReaderNeedsBeforePressing(t *testing.T) {
	good := Build([]Metric{{Name: "runway", Before: 10, After: 12, Goodness: HigherBetter}})
	if good.Worsens() {
		t.Error("an entirely positive action reported a downside")
	}
	mixed := Build([]Metric{
		{Name: "runway", Before: 10, After: 12, Goodness: HigherBetter},
		{Name: "cash", Before: 500, After: 100, Goodness: HigherBetter},
	})
	if !mixed.Worsens() {
		t.Error("an action with a cost must say so")
	}
}

func TestAnActionThatChangesNothingSaysSo(t *testing.T) {
	p := Build([]Metric{{Name: "runway", Before: 10, After: 10, Goodness: HigherBetter}})
	if p.Any() {
		t.Error("nothing moved, so nothing should be reported as changed")
	}
	if len(p.Unchanged) != 1 {
		t.Errorf("unchanged = %v, want the metric named", p.Unchanged)
	}
}

func TestUnnamedMetricsAreIgnored(t *testing.T) {
	// A row with no name is not information; it is a blank line with a number.
	p := Build([]Metric{{Before: 1, After: 2, Goodness: HigherBetter}})
	if p.Any() || len(p.Unchanged) != 0 {
		t.Errorf("an unnamed metric was reported: %+v", p)
	}
}

func TestDeltaIsAlwaysPositive(t *testing.T) {
	// The sign lives in Direction, so a surface never has to work out whether a
	// negative delta on a lower-is-better metric is good news.
	p := Build([]Metric{{Name: "debt", Before: 1000, After: 400, Goodness: LowerBetter}})
	if p.Changed[0].DeltaAbs != 600 {
		t.Errorf("delta = %d, want a positive 600", p.Changed[0].DeltaAbs)
	}
}

func TestPreviewsAreStable(t *testing.T) {
	// The same action must preview identically every time, or a reader cannot
	// re-read it to check they read it right.
	ms := []Metric{
		{Name: "zebra", Before: 1, After: 2, Goodness: HigherBetter},
		{Name: "apple", Before: 1, After: 2, Goodness: HigherBetter},
		{Name: "quiet", Before: 5, After: 5, Goodness: HigherBetter},
	}
	for i := range 5 {
		p := Build(ms)
		if p.Changed[0].Metric.Name != "apple" || p.Unchanged[0] != "quiet" {
			t.Fatalf("run %d: %+v", i, p)
		}
	}
}

func TestDisplayStringsRideAlongUntouched(t *testing.T) {
	// This package compares and never formats — formatting a change to a month
	// count and a change to money need different knowledge, and neither belongs
	// here.
	p := Build([]Metric{{
		Name: "interest", Before: 50_000, After: 30_000, Goodness: LowerBetter,
		DisplayBefore: "$500.00", DisplayAfter: "$300.00",
	}})
	if p.Changed[0].Metric.DisplayAfter != "$300.00" {
		t.Errorf("display strings were altered: %+v", p.Changed[0].Metric)
	}
}
