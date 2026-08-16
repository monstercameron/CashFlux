// SPDX-License-Identifier: MIT

// Package periodage answers one question every windowed surface needs and none
// of them were asking: how far into this period are we, and is that far enough
// for a comparison to mean anything yet?
//
// On day 3 of a month the app was confidently wrong in three places at once
// (V-sweep C344): every budget card read "$0.00 / 0% / On track", /reports
// announced "Spending is down 66% versus the previous period", and /health
// scored "Budget adherence 100%". None of those are lies about the data — they
// are lies about the *question*. Nothing had happened yet, and a surface that
// renders "nothing yet" as "doing great" trains people to ignore it, which
// costs it the one month it has something real to say.
//
// The package draws the line in one place so every surface draws it in the same
// place, and offers the two honest repairs:
//
//   - Prorate: compare like against like. A period 10% elapsed is compared to
//     the FIRST 10% of the reference period, not to all of it.
//   - Early: say so. Below EarlyFraction there is not enough of a period to
//     read, and the surface should show the last completed one instead.
//
// Pure Go, no syscall/js; unit-tested on native Go.
package periodage

import (
	"math"
	"time"
)

// EarlyFraction is how much of a period must have elapsed before a
// period-to-date figure is worth comparing or scoring.
//
// A tenth of a period is ~3 days of a month, ~9 days of a quarter — about where
// one large or one missing charge stops dominating the reading. The V-sweep's
// suggested "until ~day 5" of a month lands just above it; expressing the rule
// as a fraction rather than a day count means a weekly budget isn't held to a
// monthly budget's calendar.
const EarlyFraction = 0.1

// Age is how far a period has run, and what that permits a surface to claim.
type Age struct {
	// Start, End are the period's half-open bounds; Now is the reading time.
	Start, End, Now time.Time
	// Elapsed is the fraction of the period that has passed, in [0, 1].
	Elapsed float64
	// Day is the 1-based calendar day into the period (1 on the first day), and
	// Days is the period's whole length. Both are for phrasing — "3 days in",
	// "day 3 of 31" — never for arithmetic; use Elapsed for that.
	Day, Days int
	// Complete reports that the period has fully run, so its figures are final
	// and need no framing at all.
	Complete bool
}

// Of reads a period's age at now. Bounds are half-open [start, end), matching
// every other window in the app. A degenerate or inverted period reads as
// complete: there is no partial period to warn about.
func Of(start, end, now time.Time) Age {
	a := Age{Start: start, End: end, Now: now}
	span := end.Sub(start)
	if span <= 0 {
		a.Elapsed, a.Complete, a.Day, a.Days = 1, true, 1, 1
		return a
	}
	a.Days = max(int(math.Ceil(span.Hours()/24)), 1)
	switch {
	case !now.After(start):
		a.Elapsed, a.Day = 0, 1
	case !now.Before(end):
		a.Elapsed, a.Complete, a.Day = 1, true, a.Days
	default:
		a.Elapsed = float64(now.Sub(start)) / float64(span)
		a.Day = min(int(now.Sub(start).Hours()/24)+1, a.Days)
	}
	return a
}

// Early reports whether too little of the period has run for a period-to-date
// figure to be worth comparing or scoring. A complete period is never early.
func (a Age) Early() bool { return !a.Complete && a.Elapsed < EarlyFraction }

// Prorate scales a whole-period reference figure down to the share of the
// period that has actually elapsed, so a period-to-date number can be compared
// against it without the comparison being mostly a statement about the calendar.
//
// This is the honest fix where one exists: "spending is down 66%" was comparing
// three days against thirty-one.
func (a Age) Prorate(referenceMinor int64) int64 {
	if a.Complete || a.Elapsed >= 1 {
		return referenceMinor
	}
	if a.Elapsed <= 0 {
		return 0
	}
	return int64(math.Round(float64(referenceMinor) * a.Elapsed))
}

// ProrateWindow returns the slice of a reference period covering the same
// fraction that has elapsed of this one — the "first 10% of last month" a
// period-to-date figure should actually be compared against. The returned
// window is half-open, like every other window in the app.
//
// A complete period returns the whole reference window.
func (a Age) ProrateWindow(refStart, refEnd time.Time) (start, end time.Time) {
	span := refEnd.Sub(refStart)
	if span <= 0 {
		return refStart, refEnd
	}
	if a.Complete || a.Elapsed >= 1 {
		return refStart, refEnd
	}
	if a.Elapsed <= 0 {
		return refStart, refStart
	}
	return refStart, refStart.Add(time.Duration(float64(span) * a.Elapsed))
}
