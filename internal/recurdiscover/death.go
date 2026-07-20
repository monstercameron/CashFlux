// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// StopSignal reports an active commitment that appears to have stopped: two or
// more consecutive expected occurrences have gone unmatched past their grace
// window.
type StopSignal struct {
	CommitmentID string
	// MissedCount is how many consecutive expected occurrences are overdue past
	// the grace window.
	MissedCount int
	// LastSeen is the last matched occurrence date fed in.
	LastSeen time.Time
	// NextExpected is the first missed occurrence date (the one that should have
	// arrived first).
	NextExpected time.Time
}

// DetectStopped reports whether a commitment appears to have stopped. Given the
// last matched occurrence date, the cadence, the current time, and a grace window
// (extra days a charge may lag before it counts as missed), it steps expected
// occurrences forward from lastSeen and counts how many fall due (plus grace)
// before now. Two or more missed in a row is a "seems stopped" signal.
func DetectStopped(commitmentID string, cadence Cadence, lastSeen, now time.Time, graceDays int) (StopSignal, bool) {
	if cadence == CadenceUnknown || lastSeen.IsZero() {
		return StopSignal{}, false
	}
	cutoff := dayOf(now).AddDate(0, 0, -graceDays)
	var missed int
	var firstMissed time.Time
	exp := stepDate(cadence, lastSeen)
	for !dayOf(exp).After(cutoff) {
		if missed == 0 {
			firstMissed = dayOf(exp)
		}
		missed++
		if missed > 1000 { // safety against a pathological cadence
			break
		}
		exp = stepDate(cadence, exp)
	}
	if missed < 2 {
		return StopSignal{}, false
	}
	return StopSignal{
		CommitmentID: commitmentID,
		MissedCount:  missed,
		LastSeen:     dayOf(lastSeen),
		NextExpected: firstMissed,
	}, true
}

// stepDate advances a date by one cadence period. Weekly-family cadences add a
// fixed day count; monthly-family cadences add calendar months (via
// dateutil.AddMonths, which clamps short months). Semi-monthly alternates around
// the 1st/15th, mirroring domain.CadenceSemimonthly.
func stepDate(c Cadence, from time.Time) time.Time {
	switch c {
	case CadenceWeekly:
		return from.AddDate(0, 0, 7)
	case CadenceBiweekly:
		return from.AddDate(0, 0, 14)
	case CadenceEvery4Weeks:
		return from.AddDate(0, 0, 28)
	case CadenceSemimonthly:
		if from.Day() < 15 {
			return time.Date(from.Year(), from.Month(), 15, 0, 0, 0, 0, from.Location())
		}
		next := dateutil.AddMonths(from, 1)
		return time.Date(next.Year(), next.Month(), 1, 0, 0, 0, 0, next.Location())
	case CadenceMonthly:
		return dateutil.AddMonths(from, 1)
	case CadenceQuarterly:
		return dateutil.AddMonths(from, 3)
	case CadenceSemiannual:
		return dateutil.AddMonths(from, 6)
	case CadenceAnnual:
		return dateutil.AddMonths(from, 12)
	default:
		return dateutil.AddMonths(from, 1)
	}
}
