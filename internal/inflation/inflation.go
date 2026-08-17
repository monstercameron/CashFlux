// SPDX-License-Identifier: MIT

// Package inflation converts between future dollars and today's dollars
// (FP-T2d).
//
// Every long-horizon figure in the app — a goal's projected date, a forecast
// balance, a retirement pot, a debt payoff total — is quoted in NOMINAL money:
// the dollars of whatever year it lands in. That is arithmetically correct and
// quietly misleading, because a person reads "$412,000 in 2044" against what
// $412,000 buys today, and nobody performs the discount in their head.
//
// The ticket calls this a "cross-cutting prerequisite", which is the right
// framing: it is one small function that several surfaces need to stop
// overstating themselves. It is its own package rather than a method on any one
// engine so that a forecast does not have to import a retirement engine to
// deflate a number.
//
// Two rules:
//
//   - A zero or absent rate is the IDENTITY, not an error. Plenty of callers
//     have no inflation assumption configured, and refusing to convert would
//     force every one of them to branch. Returning the input unchanged is both
//     correct (no inflation means today's dollars are future dollars) and the
//     behaviour that keeps call sites clean.
//   - A rate at or below -100% is refused. That is not deflation, it is money
//     ceasing to have a value, and the maths goes to infinity.
//
// Pure Go, no platform dependencies.
package inflation

import "math"

// MaxYears bounds a conversion. Two hundred years past any horizon this app
// projects, and the bound stops a nonsense input producing an infinity.
const MaxYears = 200

// DefaultRatePct is the assumption used when a household has not set one.
//
// Three percent: roughly the long-run US CPI average, and — more importantly —
// a round number that reads as an assumption rather than a measurement. A
// surface using it should say so; a figure deflated by an invisible 3% is
// exactly the kind of unstated assumption this package exists to expose.
const DefaultRatePct = 3.0

// Valid reports whether a rate can be used. Below -100% is not deflation, it is
// an arithmetic hole.
func Valid(ratePct float64) bool { return ratePct > -100 && ratePct <= 1000 }

// Factor is the multiplier that turns a nominal amount `years` out into today's
// money: 1/(1+r)^years.
//
// Reports ok=false for an unusable rate or horizon, so a caller can say "we
// cannot deflate this" instead of quietly applying 1.0 and presenting a nominal
// figure as a real one — which is the exact confusion the package removes.
func Factor(ratePct float64, years float64) (float64, bool) {
	if !Valid(ratePct) || years < 0 || years > MaxYears || math.IsNaN(years) {
		return 0, false
	}
	if ratePct == 0 {
		return 1, true
	}
	return 1 / math.Pow(1+ratePct/100, years), true
}

// RealMinor converts a nominal amount `years` in the future into today's money.
//
// A zero rate returns the input unchanged — no inflation means today's dollars
// ARE future dollars — so a caller with no assumption configured needs no
// branch. An unusable rate reports ok=false rather than returning the input,
// because there the caller must not present the result as a real figure.
func RealMinor(nominalMinor int64, ratePct float64, years float64) (int64, bool) {
	f, ok := Factor(ratePct, years)
	if !ok {
		return 0, false
	}
	return int64(math.Round(float64(nominalMinor) * f)), true
}

// NominalMinor is RealMinor's inverse: what an amount in today's money will
// cost in `years` time. For pricing a future goal — "this $30,000 kitchen is
// $38,000 by the time you have saved for it".
func NominalMinor(todayMinor int64, ratePct float64, years float64) (int64, bool) {
	f, ok := Factor(ratePct, years)
	if !ok || f == 0 {
		return 0, false
	}
	return int64(math.Round(float64(todayMinor) / f)), true
}

// ErosionPct is how much purchasing power an amount loses over `years`, as a
// percent of its nominal value — the number that makes the point without asking
// anyone to compare two figures.
//
// Always in [0, 100) for a positive rate: a negative rate (deflation) gains
// power, which is reported as a negative erosion rather than clamped, because
// clamping would hide a real (if rare) direction.
func ErosionPct(ratePct float64, years float64) (float64, bool) {
	f, ok := Factor(ratePct, years)
	if !ok {
		return 0, false
	}
	return (1 - f) * 100, true
}

// YearsBetween is the fractional years from a to b, for callers holding dates
// rather than a horizon. Negative when b precedes a; callers deflating a future
// figure should pass the pair in order.
//
// 365.25 days, because a horizon measured in years and then compounded is
// sensitive to leap-year drift over decades in a way a single date difference
// is not.
func YearsBetween(daysApart float64) float64 { return daysApart / 365.25 }
