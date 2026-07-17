// SPDX-License-Identifier: MIT

package goaltrajectory

import (
	"testing"
	"time"
)

func TestProjectScenariosOrdering(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// $0 → $1,200 at $100/month: expected lands in 11 months (month-11 payment
	// crosses, counted at m-1 per the package convention).
	sc := ProjectScenarios(Input{
		CurrentMinor: 0, TargetMinor: 120000, MonthlyMinor: 10000, Start: start,
	})
	if !sc.Expected.Reachable || !sc.Conservative.Reachable || !sc.Best.Reachable {
		t.Fatalf("all scenarios should be reachable: cons=%v exp=%v best=%v",
			sc.Conservative.Reachable, sc.Expected.Reachable, sc.Best.Reachable)
	}
	// Best (125% pace) lands no later than expected; conservative (75%) no earlier.
	if sc.Best.MonthsToGoal > sc.Expected.MonthsToGoal {
		t.Errorf("best %d months > expected %d", sc.Best.MonthsToGoal, sc.Expected.MonthsToGoal)
	}
	if sc.Conservative.MonthsToGoal < sc.Expected.MonthsToGoal {
		t.Errorf("conservative %d months < expected %d", sc.Conservative.MonthsToGoal, sc.Expected.MonthsToGoal)
	}
	// Concrete paces: 7500/mo → 16 payments (m-1=15); 12500/mo → 10 payments (m-1=9).
	if got := sc.Conservative.MonthsToGoal; got != 15 {
		t.Errorf("conservative months = %d, want 15", got)
	}
	if got := sc.Best.MonthsToGoal; got != 9 {
		t.Errorf("best months = %d, want 9", got)
	}
}

func TestProjectScenariosEdges(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Already met: all three collapse to the same immediate result.
	sc := ProjectScenarios(Input{CurrentMinor: 500, TargetMinor: 500, MonthlyMinor: 100, Start: start})
	for name, r := range map[string]Result{"cons": sc.Conservative, "exp": sc.Expected, "best": sc.Best} {
		if !r.Reachable || r.MonthsToGoal != 0 {
			t.Errorf("%s: already-met goal should be immediately reachable, got %+v", name, r)
		}
	}

	// No contribution: scaling must not invent a pace.
	sc = ProjectScenarios(Input{CurrentMinor: 0, TargetMinor: 1000, MonthlyMinor: 0, Start: start})
	if sc.Conservative.Reachable || sc.Expected.Reachable || sc.Best.Reachable {
		t.Error("zero-contribution scenarios must all be unreachable")
	}

	// Tiny plan: 1 minor unit × 3/4 floors at 1, not 0.
	sc = ProjectScenarios(Input{CurrentMinor: 0, TargetMinor: 3, MonthlyMinor: 1, Start: start})
	if !sc.Conservative.Reachable {
		t.Error("floored conservative pace should still reach a tiny target")
	}
}
