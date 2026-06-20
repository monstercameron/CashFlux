// Package bills derives upcoming bills from the user's accounts and recurring
// cash flows (B22). It is a pure, derived read: each liability account that has a
// statement due-day and a minimum payment becomes a recurring monthly bill, and
// each negative recurring cash flow becomes a bill-like upcoming payment. It
// owns no store — paid/unpaid status builds on top of this.
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
		return billLess(out[i], out[j])
	})
	return out
}

// UpcomingAll returns account-derived bills plus negative recurring cash flows
// from Planning. Positive recurring items are income and are skipped. A recurring
// item whose NextDue is in the past is advanced by cadence until it lands on or
// after now, bounded to avoid bad imported schedules looping forever.
func UpcomingAll(accounts []domain.Account, recurring []domain.Recurring, now time.Time) []Bill {
	out := Upcoming(accounts, now)
	for _, r := range recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		due, ok := nextRecurringDue(r, now)
		if !ok {
			continue
		}
		out = append(out, Bill{
			AccountID: "recurring:" + r.ID,
			Name:      r.Label,
			Amount:    r.Amount.Abs(),
			DueDate:   due,
			DaysUntil: daysBetween(now, due),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return billLess(out[i], out[j])
	})
	return out
}

// AnnualAmounts returns the cadence-annualized cost of each recurring obligation,
// in its own currency: every qualifying liability's minimum payment ×12 (monthly
// statements), plus each negative recurring cash flow normalized to a yearly
// amount via its cadence (weekly ×52, monthly ×12, quarterly ×4, yearly ×1). This
// is the correct basis for a yearly total — unlike multiplying the sum of the
// current upcoming occurrences by 12, which mixes cadences. Callers FX-convert and
// sum the results (mirroring how the upcoming list is totalled).
func AnnualAmounts(accounts []domain.Account, recurring []domain.Recurring) []money.Money {
	var out []money.Money
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		if a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		mp := a.MinPayment.Abs()
		out = append(out, money.New(mp.Amount*12, mp.Currency))
	}
	for _, r := range recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		// MonthlyEquivalent already normalizes the cadence to a per-month figure;
		// ×12 yields the yearly amount. Abs since recurring outflows are negative.
		annual := r.MonthlyEquivalent() * 12
		if annual < 0 {
			annual = -annual
		}
		out = append(out, money.New(annual, r.Amount.Currency))
	}
	return out
}

func billLess(a, b Bill) bool {
	if !a.DueDate.Equal(b.DueDate) {
		return a.DueDate.Before(b.DueDate)
	}
	return a.AccountID < b.AccountID
}

func nextRecurringDue(r domain.Recurring, now time.Time) (time.Time, bool) {
	if r.ID == "" || r.Label == "" || r.NextDue.IsZero() {
		return time.Time{}, false
	}
	due := dateOnly(r.NextDue)
	ref := dateOnly(now)
	for i := 0; due.Before(ref) && i < 240; i++ {
		due = dateOnly(r.Cadence.Next(due))
	}
	if due.Before(ref) {
		return time.Time{}, false
	}
	return due, true
}

// NextDue returns the next occurrence of the given day-of-month on or after the
// calendar date of from, clamped to the month's length so a due-day past the end
// of a short month (e.g. 31 in February) lands on the last day instead of
// overflowing into the next month.
func NextDue(dueDay int, from time.Time) time.Time {
	loc := from.Location()
	fromDate := dateOnly(from)

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

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
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
