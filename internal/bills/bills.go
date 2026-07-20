// SPDX-License-Identifier: MIT

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
	"strconv"
	"strings"
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
	Autopay   bool        // C154/C157: biller charges this automatically (recurring-derived bills only)
	// AnchorAccountID is set by DedupeObligations when this bill absorbed the
	// liability-statement bill for the SAME real payment: it names the liability
	// account the payment settles, so a surface can still offer the account
	// (anchor chip, payoff drill-down) on the single merged row. Empty when the
	// bill was not a merge.
	AnchorAccountID string
}

// recurringAccountPrefix marks a Bill derived from a recurring flow rather than
// from a liability account's statement: its AccountID is "recurring:<id>".
const recurringAccountPrefix = "recurring:"

// RecurringIDFromAccount extracts the recurring-flow id from a bill's AccountID
// when the bill was derived from a recurring rule, reporting ok=false for a
// liability-statement bill (which carries a real account id).
func RecurringIDFromAccount(accountID string) (string, bool) {
	return strings.CutPrefix(accountID, recurringAccountPrefix)
}

// DedupeObligations collapses the DUAL BILL IDENTITY: a liability account's own
// statement bill and the recurring flow the household created to pay it are the
// SAME real payment, and listing both double-counts the money owed (a car loan
// showing as "Car payment (Marcus)" AND "Marcus's Car Loan" on the same day for
// the same amount).
//
// Two bills are the same obligation when they fall on the same date for the same
// amount and currency, and the recurring side repeats MONTHLY — only a monthly
// flow can mirror a monthly statement, so a weekly or quarterly flow that happens
// to coincide once is a coincidence, not a duplicate. This mirrors the
// (currency, amount, due-day) rule UpcomingAll already applies to the
// next-bill-per-account view; DedupeObligations is its equivalent for the
// multi-occurrence window OccurrencesWithin returns.
//
// The surviving row is the RECURRING one — it carries the household's own label,
// its posting mode, and the schedule that "mark paid" advances — and it records
// the liability account in AnchorAccountID so the merged row keeps both
// identities' capabilities. Input order is preserved.
func DedupeObligations(bs []Bill, recurring []domain.Recurring) []Bill {
	cadence := make(map[string]domain.RecurringCadence, len(recurring))
	for _, r := range recurring {
		cadence[r.ID] = r.Cadence
	}
	obligationKey := func(b Bill) string {
		return b.Amount.Currency + ":" + strconv.FormatInt(b.Amount.Amount, 10) + ":" + b.DueDate.Format("2006-01-02")
	}
	// Index the account-derived (statement) bills by obligation.
	statement := make(map[string][]int, len(bs))
	for i, b := range bs {
		if _, isRecurring := RecurringIDFromAccount(b.AccountID); isRecurring {
			continue
		}
		k := obligationKey(b)
		statement[k] = append(statement[k], i)
	}
	dropped := make(map[int]bool, len(bs))
	anchorOf := make(map[int]string, len(bs))
	for i, b := range bs {
		rid, isRecurring := RecurringIDFromAccount(b.AccountID)
		if !isRecurring || cadence[rid] != domain.CadenceMonthly {
			continue
		}
		for _, j := range statement[obligationKey(b)] {
			if dropped[j] {
				continue // already absorbed by an earlier occurrence
			}
			dropped[j] = true
			anchorOf[i] = bs[j].AccountID
			break
		}
	}
	out := make([]Bill, 0, len(bs))
	for i, b := range bs {
		if dropped[i] {
			continue
		}
		if anchor, ok := anchorOf[i]; ok {
			b.AnchorAccountID = anchor
		}
		out = append(out, b)
	}
	return out
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
	// A recurring flow often models the SAME real payment as its target
	// liability's own statement due-date (a car/mortgage/loan payment). Both the
	// account-derived bill and the recurring flow would otherwise list separately
	// and double-count the headline "total due" and "per year". Dedupe by
	// (currency, amount, due day-of-month): a recurring flow matching a bill
	// already surfaced from the accounts is the same obligation, so skip it and
	// keep the account's ✦ representation.
	billKey := func(cur string, minor int64, day int) string {
		return cur + ":" + strconv.FormatInt(minor, 10) + ":" + strconv.Itoa(day)
	}
	seen := make(map[string]bool, len(out))
	for _, b := range out {
		seen[billKey(b.Amount.Currency, b.Amount.Amount, b.DueDate.Day())] = true
	}
	for _, r := range recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		due, ok := nextRecurringDue(r, now)
		if !ok {
			continue
		}
		amt := r.Amount.Abs()
		// Only a MONTHLY recurring can duplicate a monthly liability statement; a
		// weekly/quarterly flow that happens to share an amount+day is a coincidence,
		// not the same obligation.
		if r.Cadence == domain.CadenceMonthly && seen[billKey(amt.Currency, amt.Amount, due.Day())] {
			continue // same obligation as an account-derived bill already listed
		}
		out = append(out, Bill{
			AccountID: "recurring:" + r.ID,
			Name:      r.Label,
			Amount:    amt,
			DueDate:   due,
			DaysUntil: daysBetween(now, due),
			Autopay:   r.Autopay,
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
	// Same dedupe rationale as UpcomingAll: a monthly recurring flow that models a
	// liability's statement payment (same currency, monthly amount, and due day)
	// is the same obligation and must not be counted twice in the yearly total.
	key := func(cur string, minor int64, day int) string {
		return cur + ":" + strconv.FormatInt(minor, 10) + ":" + strconv.Itoa(day)
	}
	seen := map[string]bool{}
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		if a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		mp := a.MinPayment.Abs()
		seen[key(mp.Currency, mp.Amount, a.DueDayOfMonth)] = true
		out = append(out, money.New(mp.Amount*12, mp.Currency))
	}
	for _, r := range recurring {
		if !r.Amount.IsNegative() {
			continue
		}
		// MonthlyEquivalent already normalizes the cadence to a per-month figure;
		// ×12 yields the yearly amount. Abs since recurring outflows are negative.
		monthly := r.MonthlyEquivalent()
		if monthly < 0 {
			monthly = -monthly
		}
		// Only a monthly recurring can be the same obligation as a monthly
		// liability statement (see UpcomingAll).
		if r.Cadence == domain.CadenceMonthly && seen[key(r.Amount.Currency, monthly, r.NextDue.Day())] {
			continue // already counted via the target liability's minimum payment
		}
		out = append(out, money.New(monthly*12, r.Amount.Currency))
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
