// SPDX-License-Identifier: MIT

package txntoolbar

import "time"

// DayPoint is one amount on a calendar day, already converted to the base currency.
type DayPoint struct {
	Day   time.Time
	Minor int64
}

// RunningTotal returns the running total at the end of each day from first through
// last, inclusive. Points before first count toward the starting total, and points
// after last are ignored. Days are compared by their calendar date, so a time of day
// on a point does not move it to another day.
func RunningTotal(points []DayPoint, first, last time.Time) []int64 {
	const layout = "2006-01-02"
	firstKey, lastKey := first.Format(layout), last.Format(layout)
	if lastKey < firstKey {
		return nil
	}
	var start int64
	perDay := make(map[string]int64, len(points))
	for _, p := range points {
		k := p.Day.Format(layout)
		switch {
		case k < firstKey:
			start += p.Minor
		case k <= lastKey:
			perDay[k] += p.Minor
		}
	}
	var out []int64
	total := start
	for day := first; day.Format(layout) <= lastKey; day = day.AddDate(0, 0, 1) {
		total += perDay[day.Format(layout)]
		out = append(out, total)
	}
	return out
}
