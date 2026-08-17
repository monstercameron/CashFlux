// SPDX-License-Identifier: MIT

// Package watchlist turns a saved transaction view into a monitor: a number
// against a target, and a number against what is normal (WF8).
//
// Saved views already capture a scope and an optional threshold, but the
// threshold is binary — crossed or not — and a binary answer arrives too late to
// act on. Somebody watching "Amazon excluding groceries" wants to know they are
// on course to overshoot while there is still a month left, not on the day it
// happens.
//
// The second number is the one that does the work. A target is a guess somebody
// made once; the trailing average is what they actually do. "You are $135 above
// your usual" needs no target at all, and it is the sentence that explains a
// figure rather than merely flagging it.
package watchlist

import "math"

// NearBandPct is how close to a target counts as "nearly there".
//
// Eighty percent. Early enough that a month still has room to change course,
// late enough that it is not firing on the third of the month — a warning that
// arrives every period is one nobody reads by the third period.
const NearBandPct = 80.0

// Signal is where a watched figure stands against its target.
type Signal string

const (
	// SignalNone means no target was set, so there is nothing to be under or over.
	//
	// Distinct from "under" on purpose: a watchlist with no target is not doing
	// well, it is not being measured, and reporting it as fine would be an answer
	// to a question nobody asked.
	SignalNone  Signal = "none"
	SignalUnder Signal = "under"
	SignalNear  Signal = "near"
	SignalOver  Signal = "over"
)

// Status is a watched view's standing.
type Status struct {
	Signal Signal
	// TotalMinor is what the view currently totals, and TargetMinor the target it
	// is measured against (zero when there is none).
	TotalMinor, TargetMinor int64
	// PctOfTarget is how far through the target the total is. Meaningless without
	// a target, so it is zero when Signal is SignalNone.
	PctOfTarget float64
	// RemainingMinor is what is left before the target, negative once past it.
	//
	// Negative-past-the-target rather than clamped: "you are $40 over" is the
	// fact, and a clamped zero would read as "you have nothing left", which is a
	// different and softer thing.
	RemainingMinor int64
}

// Evaluate places a total against a target.
//
// It compares MAGNITUDES. Spending totals are negative in this ledger and a
// target is a positive limit, so a signed comparison reads a $3,394 spend
// against a $1 alert as 339,395% UNDER — which is what it did until a browser
// check printed the number out loud. `savedtxnview.CrossedThreshold` has always
// taken the absolute value for the same reason; this now agrees with it, and the
// two must, or one screen says "over" while the other says "plenty left".
func Evaluate(totalMinor, targetMinor int64) Status {
	s := Status{TotalMinor: totalMinor, TargetMinor: targetMinor, Signal: SignalNone}
	if targetMinor <= 0 {
		return s
	}
	totalMinor = abs64(totalMinor)
	s.PctOfTarget = float64(totalMinor) / float64(targetMinor) * 100
	s.RemainingMinor = targetMinor - totalMinor
	switch {
	case totalMinor > targetMinor:
		s.Signal = SignalOver
	case s.PctOfTarget >= NearBandPct:
		s.Signal = SignalNear
	default:
		s.Signal = SignalUnder
	}
	return s
}

// MinPeriodsForAverage is how many completed periods are needed before a
// "usual" exists.
//
// Three. Two periods give an average that either of them can swing entirely, and
// calling that "normal" invents a baseline out of a coincidence.
const MinPeriodsForAverage = 3

// Comparison is this period against what the household usually does.
type Comparison struct {
	// AverageMinor is the mean of the completed prior periods.
	AverageMinor int64
	// DeltaMinor is current minus average: POSITIVE means more than usual, which
	// for spending is the direction worth noticing.
	DeltaMinor int64
	// DeltaPct is that as a share of the average.
	DeltaPct float64
	// Periods is how many prior periods fed the average, so a surface can say
	// "compared with your last four months" rather than an unattributed "usual".
	Periods int
}

// Unusual reports whether the difference is big enough to be worth a sentence.
func (c Comparison) Unusual() bool { return math.Abs(c.DeltaPct) >= UnusualPct }

// UnusualPct is how far from the average counts as worth mentioning.
//
// Twenty percent. Spending is lumpy — a month is routinely a tenth either side
// of normal without meaning anything — and a monitor that calls every ordinary
// fluctuation unusual teaches people to ignore it.
const UnusualPct = 20.0

// Compare measures the current period against completed prior ones.
//
// Reports ok=false with fewer than MinPeriodsForAverage priors, or when the
// average is zero. A percentage against a zero baseline is not a large
// percentage, it is a division nobody should have done.
func Compare(currentMinor int64, priorMinor []int64) (Comparison, bool) {
	if len(priorMinor) < MinPeriodsForAverage {
		return Comparison{}, false
	}
	var sum int64
	for _, v := range priorMinor {
		sum += v
	}
	avg := sum / int64(len(priorMinor))
	if avg == 0 {
		return Comparison{}, false
	}
	c := Comparison{
		AverageMinor: avg,
		DeltaMinor:   currentMinor - avg,
		Periods:      len(priorMinor),
	}
	c.DeltaPct = float64(c.DeltaMinor) / math.Abs(float64(avg)) * 100
	return c, true
}

// abs64 is the magnitude of a signed minor-unit amount.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
