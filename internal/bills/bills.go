// Package bills derives upcoming bills from the user's accounts (B22). For now it
// is a pure, derived read: each liability account that has a statement due-day
// and a minimum payment becomes a recurring monthly bill, with its next due date
// and days-until computed from the calendar. It owns no store — paid/unpaid
// status and the calendar UI build on top of this.
//
// Pure Go, no syscall/js; due-date math reuses the standard library calendar so
// month-end clamping (a "due on the 31st" bill in February) is handled correctly.
package bills

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Bill is one upcoming payment derived from an account.
type Bill struct {
	AccountID string
	Name      string
	Amount    money.Money // the minimum payment due (positive)
	DueDate   time.Time   // the next due date on or after the reference time
	DaysUntil int         // whole days from the reference date to DueDate (0 = due today)
}

// Upcoming returns the next bill for each active liability account that has a
// due-day and a non-zero minimum payment, soonest first (ties broken by account
// id for determinism). Archived accounts and assets are skipped. now is the
// reference time; only its calendar date is used.
func Upcoming(accounts []domain.Account, now time.Time) []Bill {
	var out []Bill
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		if a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		due := NextDue(a.DueDayOfMonth, now)
		out = append(out, Bill{
			AccountID: a.ID,
			Name:      a.Name,
			Amount:    a.MinPayment.Abs(),
			DueDate:   due,
			DaysUntil: daysBetween(now, due),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].DueDate.Equal(out[j].DueDate) {
			return out[i].DueDate.Before(out[j].DueDate)
		}
		return out[i].AccountID < out[j].AccountID
	})
	return out
}

// NextDue returns the next occurrence of the given day-of-month on or after the
// calendar date of from, clamped to the month's length so a due-day past the end
// of a short month (e.g. 31 in February) lands on the last day instead of
// overflowing into the next month.
func NextDue(dueDay int, from time.Time) time.Time {
	loc := from.Location()
	fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)

	y, m := from.Year(), from.Month()
	cand := time.Date(y, m, clampDay(y, m, dueDay), 0, 0, 0, 0, loc)
	if cand.Before(fromDate) {
		ny, nm := y, m+1
		if nm > time.December {
			ny, nm = y+1, time.January
		}
		cand = time.Date(ny, nm, clampDay(ny, nm, dueDay), 0, 0, 0, 0, loc)
	}
	return cand
}

// clampDay limits day to the number of days in the given month.
func clampDay(year int, month time.Month, day int) int {
	if max := daysInMonth(year, month); day > max {
		return max
	}
	if day < 1 {
		return 1
	}
	return day
}

// daysInMonth returns the number of days in the given month (day 0 of the next
// month is the last day of this one).
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// daysBetween returns the whole-day difference between the calendar dates of from
// and to (positive when to is later).
func daysBetween(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}
