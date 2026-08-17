// SPDX-License-Identifier: MIT

// Package goaltrajectory projects a savings goal's balance forward month by
// month from the current saved amount toward its target, assuming a fixed
// monthly contribution. It answers "if I keep saving $X/month, what does the
// balance look like over time and when do I land on the target?".
//
// Pure Go, no platform dependencies. All money is int64 minor units; the caller
// formats at the edge. Unit-tested on native Go.
package goaltrajectory

import (
	"github.com/monstercameron/CashFlux/internal/inflation"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// defaultMaxMonths bounds an otherwise open-ended projection so the series stays
// finite (ten years). Used when Input.MaxMonths is unset.
const defaultMaxMonths = 120

// flatHorizon is how many months a non-accruing (zero/negative contribution)
// projection spans, so the chart still draws a short, honest flat line rather
// than a single dot. A future target date widens it up to the deadline.
const flatHorizon = 6

// Input describes a savings goal to project. Amounts are int64 minor units in a
// single currency (the caller keeps currency consistent). Start is the "as of"
// date the projection begins from. TargetDate is optional (zero = none) and
// widens an unreachable projection's horizon to the deadline. MaxMonths caps the
// series length; zero selects the default.
type Input struct {
	CurrentMinor int64
	TargetMinor  int64
	MonthlyMinor int64
	Start        time.Time
	TargetDate   time.Time // zero = none
	MaxMonths    int
}

// Point is one month on the projected balance curve.
type Point struct {
	Month        time.Time
	BalanceMinor int64
}

// Result is a projected trajectory. Series always holds at least one point
// (month zero at the current balance). MonthsToGoal is the whole months from
// Start until the balance first reaches the target (0 when already met).
// Reachable reports whether the target is met within the month cap. ProjectedDate
// is the landing month (zero when not reachable).
type Result struct {
	Series        []Point
	MonthsToGoal  int
	Reachable     bool
	ProjectedDate time.Time
}

// Project builds the monthly balance trajectory for a goal.
//
// Behavior:
//   - Already at/over target: a single month-zero point, MonthsToGoal 0,
//     Reachable true, ProjectedDate = Start.
//   - Positive monthly contribution: accrues month by month until the balance
//     reaches the target (Reachable, MonthsToGoal set) or the month cap is hit
//     (Reachable false).
//   - Zero/negative monthly contribution: a short flat series at the current
//     balance, Reachable false (the target is never approached). A future target
//     date widens the flat horizon to the deadline.
func Project(in Input) Result {
	maxMonths := in.MaxMonths
	if maxMonths <= 0 {
		maxMonths = defaultMaxMonths
	}

	// Already met: nothing to project — one point at the current balance.
	if in.CurrentMinor >= in.TargetMinor {
		return Result{
			Series:        []Point{{Month: in.Start, BalanceMinor: in.CurrentMinor}},
			MonthsToGoal:  0,
			Reachable:     true,
			ProjectedDate: in.Start,
		}
	}

	// No forward motion: draw a short flat line so the chart reads "not moving
	// toward the target" rather than reaching it. Never marked reachable.
	if in.MonthlyMinor <= 0 {
		horizon := flatHorizon
		if h := monthsUntil(in.Start, in.TargetDate); h > 0 {
			horizon = h
		}
		if horizon > maxMonths {
			horizon = maxMonths
		}
		if horizon < 1 {
			horizon = 1
		}
		series := make([]Point, 0, horizon+1)
		for m := 0; m <= horizon; m++ {
			series = append(series, Point{Month: dateutil.AddMonths(in.Start, m), BalanceMinor: in.CurrentMinor})
		}
		return Result{Series: series, Reachable: false}
	}

	// Positive contribution: accrue month by month until the target is reached or
	// the cap is hit. Month zero seeds the current balance.
	series := make([]Point, 0, 16)
	series = append(series, Point{Month: in.Start, BalanceMinor: in.CurrentMinor})
	bal := in.CurrentMinor
	monthsToGoal := 0
	reachable := false
	for m := 1; m <= maxMonths; m++ {
		bal += in.MonthlyMinor
		series = append(series, Point{Month: dateutil.AddMonths(in.Start, m), BalanceMinor: bal})
		if bal >= in.TargetMinor {
			// Contributions land during the CURRENT month first (the goals
			// package's MonthlyNeeded convention), so the m-th payment — the
			// one that crosses the target — happens in month m-1. Counting it
			// a month later made a goal paying exactly its suggested monthly
			// read "1 mo behind" whenever the deadline fell mid-month.
			monthsToGoal = m - 1
			reachable = true
			break
		}
	}

	res := Result{Series: series, MonthsToGoal: monthsToGoal, Reachable: reachable}
	if reachable {
		res.ProjectedDate = dateutil.AddMonths(in.Start, monthsToGoal)
	}
	return res
}

// monthsUntil returns the whole months from start to date (rounding a partial
// final month up), or 0 when date is zero or not after start. Mirrors the goals
// package's month-counting convention so horizons line up with pace figures.
func monthsUntil(start, date time.Time) int {
	if date.IsZero() || !date.After(start) {
		return 0
	}
	months := (date.Year()-start.Year())*12 + int(date.Month()) - int(start.Month())
	if date.Day() > start.Day() {
		months++
	}
	if months < 0 {
		months = 0
	}
	return months
}

// ─── FP-T2d: what the target is worth when you get there ─────────────────────

// RealTargetMinor reports what a goal's target amount is worth in TODAY's money
// at the moment it is projected to be reached, and whether the question has an
// answer.
//
// This is the cross-cutting point of the inflation work: a goal to save $30,000
// for a kitchen, reached in eight years, is not a $30,000 kitchen. The target is
// stated in today's money and reached in future money, and nothing in the app
// closed that gap — so a long-dated goal quietly overstated what it would buy.
//
// ok is false when the goal is not reachable (there is no date to discount to)
// or the rate is unusable. A caller must not fall back to the nominal figure and
// present it as real; that is the confusion this exists to remove.
func (r Result) RealTargetMinor(targetMinor int64, ratePct float64) (int64, bool) {
	if !r.Reachable || r.MonthsToGoal < 0 {
		return 0, false
	}
	return inflation.RealMinor(targetMinor, ratePct, float64(r.MonthsToGoal)/12)
}

// ShortfallMinor is how much of a goal's stated target inflation eats before it
// is reached — the target minus what it will actually be worth.
//
// Zero-or-better is a legitimate answer (a goal reached this month loses
// nothing), so the figure is not clamped: a negative value under deflation is a
// real, if unusual, gain and hiding it would be a small lie for tidiness.
func (r Result) ShortfallMinor(targetMinor int64, ratePct float64) (int64, bool) {
	real, ok := r.RealTargetMinor(targetMinor, ratePct)
	if !ok {
		return 0, false
	}
	return targetMinor - real, true
}
