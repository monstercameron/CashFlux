// SPDX-License-Identifier: MIT

package goaltrajectory

import "time"

// CompareSide identifies which of two compared goals a verdict favours.
type CompareSide int

const (
	// SideNone: the two goals are equal on this dimension, or it can't be compared
	// (a projected landing is unavailable, or a monthly plan is missing).
	SideNone CompareSide = iota
	// SideA: the first goal.
	SideA
	// SideB: the second goal.
	SideB
)

// CompareInput holds the two goals' projected landings and monthly plans — exactly the
// figures the Compare table already shows. Projected/Reachable come from Project();
// Reachable=false means the goal has no projected landing, so the timing half of the
// verdict is unavailable. Monthly amounts are minor units in a SHARED currency (the
// caller only fills MonthlyKnown when both goals use the same currency, since a
// cross-currency monthly gap is meaningless).
type CompareInput struct {
	AProjected, BProjected time.Time
	AReachable, BReachable bool
	AMonthlyMinor          int64
	BMonthlyMinor          int64
	MonthlyKnown           bool // both goals have a monthly plan in the same currency
}

// Comparison summarises how two goals' CURRENT plans relate: which lands sooner and by
// how many whole months, and which carries the heavier monthly commitment and by how
// much. Every field is derived only from CompareInput — the same figures already on the
// table — so the one-sentence verdict can never disagree with the rows.
type Comparison struct {
	// Sooner is the goal projected to finish first (SideNone when the timing can't be
	// compared or both land the same month). MonthsApart is the whole-month gap (0 when
	// same month or not comparable).
	Sooner      CompareSide
	MonthsApart int
	// Costlier is the goal whose monthly plan is larger (SideNone when equal or a plan is
	// missing). MonthlyGapMinor is the absolute difference in minor units.
	Costlier        CompareSide
	MonthlyGapMinor int64
	// SameTiming is true when both goals are reachable and land in the same month — the
	// timing IS comparable, it's simply a tie (distinct from "not comparable").
	SameTiming bool
}

// Meaningful reports whether the comparison has anything worth stating: a timing
// difference/tie or a monthly-cost difference. When false, the caller shows no verdict.
func (c Comparison) Meaningful() bool {
	return c.Sooner != SideNone || c.SameTiming || c.Costlier != SideNone
}

// Compare computes the relationship between two goals' plans for the Compare verdict.
func Compare(in CompareInput) Comparison {
	var c Comparison

	if in.AReachable && in.BReachable && !in.AProjected.IsZero() && !in.BProjected.IsZero() {
		// months from A's landing to B's landing: positive ⇒ B lands later ⇒ A sooner.
		d := monthIndex(in.BProjected) - monthIndex(in.AProjected)
		switch {
		case d > 0:
			c.Sooner, c.MonthsApart = SideA, d
		case d < 0:
			c.Sooner, c.MonthsApart = SideB, -d
		default:
			c.SameTiming = true
		}
	}

	if in.MonthlyKnown {
		g := in.AMonthlyMinor - in.BMonthlyMinor
		switch {
		case g > 0:
			c.Costlier, c.MonthlyGapMinor = SideA, g
		case g < 0:
			c.Costlier, c.MonthlyGapMinor = SideB, -g
		}
	}

	return c
}

// monthIndex maps a date to a whole-month ordinal (year·12 + month) so two landings can
// be differenced in whole months regardless of day-of-month.
func monthIndex(t time.Time) int {
	return t.Year()*12 + int(t.Month()) - 1
}
