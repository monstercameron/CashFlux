// SPDX-License-Identifier: MIT

package reports

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// dayStart returns midnight (in t's location) of t's calendar date.
func dayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// NoSpendDays counts the elapsed days in the half-open period [start, end) on
// which no expense occurred — a motivating "you went N days without spending"
// metric. Only days that have actually happened are counted: the window is capped
// at the end of `now`'s day, so future days in the current period don't inflate
// the figure. Transfers and income don't count as spending. Returns 0 for an
// empty or entirely-future window.
func NoSpendDays(txns []domain.Transaction, start, end, now time.Time) int {
	if !end.After(start) {
		return 0
	}
	limit := end
	if todayEnd := dayStart(now).AddDate(0, 0, 1); todayEnd.Before(limit) {
		limit = todayEnd
	}

	spent := map[string]bool{}
	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		spent[t.Date.Format("2006-01-02")] = true
	}

	count := 0
	for d := dayStart(start); d.Before(limit); d = d.AddDate(0, 0, 1) {
		if !spent[d.Format("2006-01-02")] {
			count++
		}
	}
	return count
}
