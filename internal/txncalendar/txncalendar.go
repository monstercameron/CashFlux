// SPDX-License-Identifier: MIT

// Package txncalendar buckets transactions into calendar days and projects
// recurring cash flows forward as "ghost" occurrences, so the /transactions
// calendar view mode can render a month grid whose cells show each day's net
// amount, transaction-count density, and the bills due that day.
//
// It is a pure PROJECTION of whatever set of transactions it is handed — the
// caller passes the already-filtered ledger, so active filter chips continue to
// scope the calendar exactly as they scope the table. Pure Go, no syscall/js;
// unit-tested on native Go.
package txncalendar

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// DayKey returns the canonical calendar-day key (YYYY-MM-DD, UTC) used to bucket
// a transaction or a ghost onto a day. Transaction dates are UTC-midnight
// calendar dates (dateutil), so bucketing on the UTC date keeps a day cell in
// step with the ledger row's rendered date.
func DayKey(t time.Time) string {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// DayStat aggregates the transactions that fall on one calendar day: the signed
// net amount in minor units (income positive, spend negative) and the count of
// transactions (the dot-density source).
type DayStat struct {
	Net   int64
	Count int
}

// BucketByDay groups txns by calendar day, summing each day's net minor-unit
// amount and counting its transactions. Amounts are summed in their stored minor
// units without FX conversion — the calendar is a single-currency household
// glance; a caller formats the total with the base currency. The returned map is
// keyed by DayKey.
func BucketByDay(txns []domain.Transaction) map[string]DayStat {
	out := make(map[string]DayStat)
	for _, t := range txns {
		k := DayKey(t.Date)
		s := out[k]
		s.Net += t.Amount.Amount
		s.Count++
		out[k] = s
	}
	return out
}

// Ghost is a projected recurring occurrence on a due date — a dimmed, read-only
// entry in a day cell that names the recurring and its signed amount. It carries
// no interaction beyond a title/tooltip.
type Ghost struct {
	RecurringID string
	Label       string
	Amount      int64 // signed minor units (negative = money out)
	Date        time.Time
}

// maxGhostSteps bounds occurrence expansion so a degenerate imported cadence
// (one whose Next never advances) cannot loop forever. A visible month spans at
// most 31 days, so even a daily recurring fits well within this.
const maxGhostSteps = 400

// Ghosts projects every recurring's occurrences into the half-open range
// [from, to), stepping from each recurring's NextDue by its cadence. A recurring
// whose NextDue is zero (never scheduled) or before the range is stepped forward
// until it enters the window. Iteration is bounded (maxGhostSteps) against a
// non-advancing cadence.
func Ghosts(recurring []domain.Recurring, from, to time.Time) []Ghost {
	var out []Ghost
	for _, r := range recurring {
		due := r.NextDue
		if due.IsZero() {
			continue
		}
		// Advance into the window if the next due date precedes it.
		steps := 0
		for due.Before(from) && steps < maxGhostSteps {
			next := r.Cadence.Next(due)
			if !next.After(due) {
				break // non-advancing cadence — stop
			}
			due = next
			steps++
		}
		for due.Before(to) && steps < maxGhostSteps {
			out = append(out, Ghost{
				RecurringID: r.ID,
				Label:       r.Label,
				Amount:      r.Amount.Amount,
				Date:        due,
			})
			next := r.Cadence.Next(due)
			if !next.After(due) {
				break
			}
			due = next
			steps++
		}
	}
	return out
}

// Cell is one day square in the month grid. InMonth is false for the leading and
// trailing padding days that fill the first and last weeks (days belonging to the
// adjacent months). Stat holds that day's net + count; Ghosts are the recurrings
// due that day.
type Cell struct {
	Date    time.Time
	InMonth bool
	Stat    DayStat
	Ghosts  []Ghost
}

// Month builds the calendar grid for the month containing anchor: a slice of
// weeks, each exactly seven Cells, padded so the first cell is the weekStart on or
// before the first of the month and the last week is filled to the weekStart
// boundary. Each in-month day is populated with its transaction stat (from txns)
// and its ghost occurrences (from recurring). Padding days carry no stat or
// ghosts. Both txns and recurring should already be scoped to the caller's active
// filter — the grid is a pure projection of what it's given.
func Month(anchor time.Time, weekStart time.Weekday, txns []domain.Transaction, recurring []domain.Recurring) [][]Cell {
	monthStart := dateutil.MonthStart(anchor)
	monthEnd := dateutil.AddMonths(monthStart, 1) // exclusive

	gridStart := dateutil.WeekStart(monthStart, weekStart)
	// Last day of the month, then the end of its week (exclusive grid end).
	lastDay := monthEnd.AddDate(0, 0, -1)
	gridEnd := dateutil.WeekStart(lastDay, weekStart).AddDate(0, 0, 7)

	stats := BucketByDay(txns)
	ghosts := Ghosts(recurring, monthStart, monthEnd)
	ghostsByDay := make(map[string][]Ghost, len(ghosts))
	for _, g := range ghosts {
		k := DayKey(g.Date)
		ghostsByDay[k] = append(ghostsByDay[k], g)
	}

	var weeks [][]Cell
	for day := gridStart; day.Before(gridEnd); {
		week := make([]Cell, 0, 7)
		for i := 0; i < 7; i++ {
			k := DayKey(day)
			inMonth := !day.Before(monthStart) && day.Before(monthEnd)
			cell := Cell{Date: day, InMonth: inMonth}
			if inMonth {
				cell.Stat = stats[k]
				cell.Ghosts = ghostsByDay[k]
			}
			week = append(week, cell)
			day = day.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
	}
	return weeks
}
