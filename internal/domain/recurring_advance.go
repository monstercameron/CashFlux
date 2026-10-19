// SPDX-License-Identifier: MIT

package domain

import "time"

// Next returns the date one cadence step after from. Month-based cadences
// clamp to the same day where possible; weekly adds seven days. An unknown
// cadence is treated as monthly.
func (c RecurringCadence) Next(from time.Time) time.Time {
	switch c {
	case CadenceDaily:
		return from.AddDate(0, 0, 1)
	case CadenceWeekly:
		return from.AddDate(0, 0, 7)
	case CadenceBiweekly:
		return from.AddDate(0, 0, 14)
	case CadenceSemimonthly:
		if from.Day() < 15 {
			return time.Date(from.Year(), from.Month(), 15, from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
		}
		return monthDate(from, 1, 1)
	case CadenceQuarterly:
		return monthDate(from, 3, from.Day())
	case CadenceYearly:
		return monthDate(from, 12, from.Day())
	default:
		return monthDate(from, 1, from.Day())
	}
}

// Prev returns the date one cadence step before from. It mirrors Next's
// calendar arithmetic and clamps to the same day where possible.
func (c RecurringCadence) Prev(from time.Time) time.Time {
	switch c {
	case CadenceDaily:
		return from.AddDate(0, 0, -1)
	case CadenceWeekly:
		return from.AddDate(0, 0, -7)
	case CadenceBiweekly:
		return from.AddDate(0, 0, -14)
	case CadenceSemimonthly:
		if from.Day() >= 15 {
			return time.Date(from.Year(), from.Month(), 1, from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
		}
		return monthDate(from, -1, 15)
	case CadenceQuarterly:
		return monthDate(from, -3, from.Day())
	case CadenceYearly:
		return monthDate(from, -12, from.Day())
	default:
		return monthDate(from, -1, from.Day())
	}
}

// monthDate advances or retreats by calendar months and clamps the requested
// day to the last day of the destination month. The requested day remains the
// current date's day because Recurring has no persisted original anchor day.
func monthDate(from time.Time, months, anchorDay int) time.Time {
	destination := time.Date(from.Year(), from.Month()+time.Month(months), 1, from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
	if last := daysInMonth(destination.Year(), destination.Month()); anchorDay > last {
		anchorDay = last
	}
	return time.Date(destination.Year(), destination.Month(), anchorDay, from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// Advance returns a copy with NextDue moved one cadence forward, as used after
// a due occurrence is posted.
func (r Recurring) Advance() Recurring {
	r.NextDue = r.Cadence.Next(r.NextDue)
	return r
}
