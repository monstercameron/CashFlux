// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// ResolveWindowChecked resolves the same calendar windows as ResolveWindow but
// rejects malformed supplied dates, unknown presets and reversed ranges.
func ResolveWindowChecked(preset, from, to string, now time.Time) (AskedWindow, error) {
	if now.IsZero() {
		return AskedWindow{}, fmt.Errorf("report reference time is required")
	}
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	for _, raw := range []string{from, to} {
		if raw != "" {
			if _, err := dateutil.ParseDate(raw); err != nil {
				return AskedWindow{}, fmt.Errorf("invalid report date %q: %w", raw, err)
			}
		}
	}
	if from == "" && to == "" {
		switch strings.ToLower(strings.TrimSpace(preset)) {
		case "", "this_month", "last_month", "this_quarter", "last_quarter", "this_year", "last_year", "year_to_date", "last_12_months", "all":
		default:
			return AskedWindow{}, fmt.Errorf("unknown report period %q", preset)
		}
	}
	w := ResolveWindow(preset, from, to, now)
	if !w.Start.IsZero() && !w.End.After(w.Start) {
		return AskedWindow{}, fmt.Errorf("report end date precedes start date")
	}
	return w, nil
}

// MonthlyBoundsChecked partitions the exact half-open requested window at
// calendar-month boundaries. For all-time windows earliest supplies the first
// recorded date. A cap is an explicit error, never a truncated financial series.
func MonthlyBoundsChecked(w AskedWindow, earliest time.Time, maxBuckets int) ([]time.Time, error) {
	start := w.Start
	if start.IsZero() {
		start = earliest
	}
	if start.IsZero() || w.End.IsZero() || !w.End.After(start) {
		return nil, fmt.Errorf("report monthly bounds require an ascending nonempty range")
	}
	if maxBuckets < 1 {
		return nil, fmt.Errorf("report monthly bucket limit must be positive")
	}
	bounds := []time.Time{start}
	for cursor := start; cursor.Before(w.End); {
		if len(bounds) > maxBuckets {
			return nil, fmt.Errorf("report series exceeds %d monthly buckets; request a narrower from/to range", maxBuckets)
		}
		next := dateutil.AddMonths(dateutil.MonthStart(cursor), 1)
		if !next.Before(w.End) {
			next = w.End
		}
		bounds = append(bounds, next)
		cursor = next
	}
	return bounds, nil
}

// HasPartialMonths reports whether either endpoint cuts a calendar month.
func HasPartialMonths(bounds []time.Time) bool {
	return len(bounds) >= 2 && (!bounds[0].Equal(dateutil.MonthStart(bounds[0])) || !bounds[len(bounds)-1].Equal(dateutil.MonthStart(bounds[len(bounds)-1])))
}
