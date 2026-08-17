// SPDX-License-Identifier: MIT

package goaltrajectory

import (
	"testing"
	"time"
)

// start is a fixed reference date used across the table so month math is stable.
var start = time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)

func TestProject(t *testing.T) {
	tests := []struct {
		name             string
		in               Input
		wantLen          int
		wantMonths       int
		wantReachable    bool
		wantLastBalance  int64
		wantProjectedNil bool
	}{
		{
			name:            "normal accrual to target",
			in:              Input{CurrentMinor: 0, TargetMinor: 100_000, MonthlyMinor: 25_000, Start: start},
			wantLen:         5, // month 0..4 (0, 25k, 50k, 75k, 100k)
			wantMonths:      3, // 4 payments starting THIS month -> done in month 3
			wantReachable:   true,
			wantLastBalance: 100_000,
		},
		{
			name:             "already met goal",
			in:               Input{CurrentMinor: 120_000, TargetMinor: 100_000, MonthlyMinor: 5_000, Start: start},
			wantLen:          1,
			wantMonths:       0,
			wantReachable:    true,
			wantLastBalance:  120_000,
			wantProjectedNil: false, // projects to Start
		},
		{
			name:             "exactly at target counts as met",
			in:               Input{CurrentMinor: 100_000, TargetMinor: 100_000, MonthlyMinor: 5_000, Start: start},
			wantLen:          1,
			wantMonths:       0,
			wantReachable:    true,
			wantLastBalance:  100_000,
			wantProjectedNil: false,
		},
		{
			name:             "zero contribution is unreachable, flat short series",
			in:               Input{CurrentMinor: 20_000, TargetMinor: 100_000, MonthlyMinor: 0, Start: start},
			wantLen:          flatHorizon + 1,
			wantMonths:       0,
			wantReachable:    false,
			wantLastBalance:  20_000, // flat — never grows
			wantProjectedNil: true,
		},
		{
			name:             "negative contribution is unreachable, flat",
			in:               Input{CurrentMinor: 20_000, TargetMinor: 100_000, MonthlyMinor: -5_000, Start: start},
			wantLen:          flatHorizon + 1,
			wantMonths:       0,
			wantReachable:    false,
			wantLastBalance:  20_000,
			wantProjectedNil: true,
		},
		{
			name:            "exact-month landing",
			in:              Input{CurrentMinor: 0, TargetMinor: 1_200, MonthlyMinor: 100, Start: start},
			wantLen:         13, // month 0..12
			wantMonths:      11, // 12 payments starting THIS month -> done in month 11
			wantReachable:   true,
			wantLastBalance: 1_200,
		},
		{
			name:            "final month overshoots target (kept honest)",
			in:              Input{CurrentMinor: 0, TargetMinor: 100, MonthlyMinor: 30, Start: start},
			wantLen:         5, // 0,30,60,90,120 series shape
			wantMonths:      3, // the 4th payment lands during month 3
			wantReachable:   true,
			wantLastBalance: 120,
		},
		{
			name:             "MaxMonths cap makes a slow goal unreachable",
			in:               Input{CurrentMinor: 0, TargetMinor: 1_000_000, MonthlyMinor: 1, Start: start, MaxMonths: 6},
			wantLen:          7, // month 0..6, never reaches
			wantMonths:       0,
			wantReachable:    false,
			wantLastBalance:  6,
			wantProjectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Project(tt.in)
			if len(got.Series) != tt.wantLen {
				t.Errorf("Series len = %d, want %d", len(got.Series), tt.wantLen)
			}
			if got.MonthsToGoal != tt.wantMonths {
				t.Errorf("MonthsToGoal = %d, want %d", got.MonthsToGoal, tt.wantMonths)
			}
			if got.Reachable != tt.wantReachable {
				t.Errorf("Reachable = %v, want %v", got.Reachable, tt.wantReachable)
			}
			if len(got.Series) > 0 {
				last := got.Series[len(got.Series)-1].BalanceMinor
				if last != tt.wantLastBalance {
					t.Errorf("last balance = %d, want %d", last, tt.wantLastBalance)
				}
			}
			// Month-zero point always seeds the current balance at Start.
			if len(got.Series) > 0 {
				if got.Series[0].BalanceMinor != tt.in.CurrentMinor {
					t.Errorf("Series[0] balance = %d, want current %d", got.Series[0].BalanceMinor, tt.in.CurrentMinor)
				}
				if !got.Series[0].Month.Equal(tt.in.Start) {
					t.Errorf("Series[0] month = %v, want Start %v", got.Series[0].Month, tt.in.Start)
				}
			}
			if tt.wantProjectedNil {
				if !got.ProjectedDate.IsZero() {
					t.Errorf("ProjectedDate = %v, want zero", got.ProjectedDate)
				}
			} else if tt.wantReachable {
				wantDate := start.AddDate(0, tt.wantMonths, 0)
				if !got.ProjectedDate.Equal(wantDate) {
					t.Errorf("ProjectedDate = %v, want %v", got.ProjectedDate, wantDate)
				}
			}
		})
	}
}

// TestProjectDefaultMaxMonths verifies the series is bounded to the default cap
// when MaxMonths is unset and the goal cannot realistically be reached.
func TestProjectDefaultMaxMonths(t *testing.T) {
	got := Project(Input{CurrentMinor: 0, TargetMinor: 1_000_000_000, MonthlyMinor: 1, Start: start})
	if got.Reachable {
		t.Fatalf("Reachable = true, want false (target unreachable within cap)")
	}
	if len(got.Series) != defaultMaxMonths+1 {
		t.Errorf("Series len = %d, want %d (default cap + month zero)", len(got.Series), defaultMaxMonths+1)
	}
}

// TestProjectTargetDateVsPace confirms that the projection is pace-driven: the
// projected landing date comes from the monthly contribution, independent of the
// goal's TargetDate. A future target date only widens an unreachable (flat)
// projection's horizon; it never overrides a reachable pace date.
func TestProjectTargetDateVsPace(t *testing.T) {
	// Pace reaches in 4 months regardless of a far-off target date.
	targetDate := start.AddDate(0, 24, 0)
	got := Project(Input{CurrentMinor: 0, TargetMinor: 100_000, MonthlyMinor: 25_000, Start: start, TargetDate: targetDate})
	if !got.Reachable || got.MonthsToGoal != 3 {
		t.Fatalf("MonthsToGoal = %d reachable=%v, want 3/true (4 payments starting this month, pace-driven)", got.MonthsToGoal, got.Reachable)
	}
	wantDate := start.AddDate(0, 3, 0)
	if !got.ProjectedDate.Equal(wantDate) {
		t.Errorf("ProjectedDate = %v, want pace date %v", got.ProjectedDate, wantDate)
	}

	// Zero contribution with a future target date: flat series widened to the
	// deadline's month horizon, still unreachable.
	flat := Project(Input{CurrentMinor: 10_000, TargetMinor: 100_000, MonthlyMinor: 0, Start: start, TargetDate: start.AddDate(0, 10, 0)})
	if flat.Reachable {
		t.Fatalf("Reachable = true, want false for zero contribution")
	}
	if len(flat.Series) != 11 { // month 0..10
		t.Errorf("flat series len = %d, want 11 (widened to target-date horizon)", len(flat.Series))
	}

	// A past target date does not widen the horizon — falls back to the short default.
	past := Project(Input{CurrentMinor: 10_000, TargetMinor: 100_000, MonthlyMinor: 0, Start: start, TargetDate: start.AddDate(0, -3, 0)})
	if len(past.Series) != flatHorizon+1 {
		t.Errorf("past-target series len = %d, want %d (no widening)", len(past.Series), flatHorizon+1)
	}
}

// ─── FP-T2d: the target is worth less when you get there ─────────────────────

// A goal to save $30,000 for a kitchen, reached in eight years, is not a $30,000
// kitchen. The target is stated in today's money and reached in future money,
// and nothing closed that gap.
func TestRealTargetIsDiscountedToTheProjectedDate(t *testing.T) {
	res := Project(Input{
		CurrentMinor: 0, TargetMinor: 3000000, MonthlyMinor: 31250,
		Start: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	})
	if !res.Reachable {
		t.Fatalf("the goal is not reachable: %+v", res)
	}
	real, ok := res.RealTargetMinor(3000000, 3)
	if !ok {
		t.Fatal("RealTargetMinor reported unknown for a reachable goal")
	}
	if real >= 3000000 {
		t.Errorf("real target = %d, not below the nominal 3000000 under 3%% inflation", real)
	}
	short, ok := res.ShortfallMinor(3000000, 3)
	if !ok || short <= 0 {
		t.Errorf("ShortfallMinor = %d,%v want a positive erosion", short, ok)
	}
	if short != 3000000-real {
		t.Errorf("shortfall %d != target-real %d", short, 3000000-real)
	}
}

// With no inflation assumption the real figure IS the target — a caller with
// nothing configured needs no branch.
func TestRealTargetWithNoInflationIsTheTarget(t *testing.T) {
	res := Project(Input{TargetMinor: 100000, MonthlyMinor: 100000,
		Start: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)})
	got, ok := res.RealTargetMinor(100000, 0)
	if !ok || got != 100000 {
		t.Errorf("RealTargetMinor at 0%% = %d,%v want the target unchanged", got, ok)
	}
}

// An unreachable goal has no date to discount to. Falling back to the nominal
// figure and presenting it as real is the confusion this exists to remove.
func TestRealTargetRefusesWhenThereIsNoDate(t *testing.T) {
	res := Project(Input{CurrentMinor: 0, TargetMinor: 3000000, MonthlyMinor: 0,
		Start: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)})
	if res.Reachable {
		t.Fatal("a zero-contribution goal reported reachable")
	}
	if _, ok := res.RealTargetMinor(3000000, 3); ok {
		t.Error("an unreachable goal produced a real target")
	}
	if _, ok := res.ShortfallMinor(3000000, 3); ok {
		t.Error("an unreachable goal produced a shortfall")
	}
}

// An unusable rate must refuse too, rather than silently returning the nominal.
func TestRealTargetRefusesAnUnusableRate(t *testing.T) {
	res := Project(Input{TargetMinor: 100000, MonthlyMinor: 100000,
		Start: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)})
	if _, ok := res.RealTargetMinor(100000, -150); ok {
		t.Error("an impossible rate produced a figure")
	}
}
