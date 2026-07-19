// SPDX-License-Identifier: MIT

// Package acctseries computes the day-window sizes for the account-detail
// balance chart's range picker (90 days / 12 months / all). The series values
// themselves come from internal/accountflow.BalanceSeries; this package only
// decides how many daily points each range spans, so the sizing is pure and
// unit-tested without a DOM.
package acctseries

import "time"

// Fixed short-window sizes, in daily points.
const (
	// Days90 is the default 90-day window (matches the row sparkline).
	Days90 = 90
	// Days12m is the rolling twelve-month window (365 daily points).
	Days12m = 365
)

// AllDays returns the number of daily points the "all" range should span: from
// the earliest of the supplied dates through asOf (inclusive), never fewer than
// floor and never more than maxDays (maxDays <= 0 disables the cap). Zero-valued
// dates and any date on or after asOf are ignored, so a household with no history
// simply gets the floor. The span is measured in whole calendar days.
func AllDays(asOf time.Time, floor, maxDays int, dates ...time.Time) int {
	days := floor
	for _, d := range dates {
		if d.IsZero() || !d.Before(asOf) {
			continue
		}
		span := int(asOf.Sub(d).Hours()/24) + 1
		if span > days {
			days = span
		}
	}
	if maxDays > 0 && days > maxDays {
		days = maxDays
	}
	return days
}

// HasRange reports whether the range picker is worth showing: only when the
// account has genuinely more than the default 90-day window of history, so a
// brand-new account keeps the plain single sparkline (no empty toggle).
func HasRange(allDays int) bool { return allDays > Days90 }
