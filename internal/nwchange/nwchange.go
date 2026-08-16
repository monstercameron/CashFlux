// SPDX-License-Identifier: MIT

// Package nwchange is the single definition of "how far has net worth moved".
//
// Before this package four surfaces answered that question four ways: the
// dashboard hero compared today's all-transactions net worth against the first
// of the month, /accounts compared two cutoffs, /networth's trend card reported
// the step between the two most recent month boundaries (i.e. LAST month), and
// /reports reported the whole report period — and all four printed the words
// "this month". A reader who visited two of them saw the same question given
// two answers, which is the fastest way to lose trust in every other figure on
// the page (V-sweep C341).
//
// One computation, one window type, one label. A window is half-open
// [Since, Until) in the app's canonical convention and every end is read with
// ledger.NetWorthSeries, so "net worth as of a cutoff" means exactly what it
// means everywhere else in the app: transactions strictly before the cutoff.
//
// The two windows that matter are named rather than assembled by callers:
// MonthToDate (the first of this month → end of today) and PriorMonth (the
// month before that, whole). A surface that wants a different window builds one
// explicitly and must label it — Change carries its own bounds so no view can
// print a delta without being able to say which window produced it.
//
// Pure Go, no syscall/js; unit-tested on native Go.
package nwchange

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Window is a half-open comparison window [Since, Until): net worth is read as
// of each bound, counting transactions strictly before it.
type Window struct {
	Since, Until time.Time
}

// MonthToDate is the first instant of now's month through the end of today.
//
// Until is tomorrow's midnight so everything posted today counts and nothing
// dated in the future does — a delta must describe money that has actually
// landed, not a schedule.
func MonthToDate(now time.Time) Window {
	return Window{
		Since: dateutil.MonthStart(now),
		Until: dateutil.DayStart(now).AddDate(0, 0, 1),
	}
}

// PriorMonth is the whole calendar month before now's month. It is the honest
// comparison to show early in a period, when month-to-date has barely any
// activity in it (V-sweep C344).
func PriorMonth(now time.Time) Window {
	cur := dateutil.MonthStart(now)
	return Window{Since: dateutil.AddMonths(cur, -1), Until: cur}
}

// Months is the window covering the last n whole calendar months up to the end
// of today (n = 1 is the same span as MonthToDate). It exists so a trend card
// can state its own window instead of borrowing the month-to-date label.
func Months(now time.Time, n int) Window {
	if n < 1 {
		n = 1
	}
	return Window{
		Since: dateutil.AddMonths(dateutil.MonthStart(now), -(n - 1)),
		Until: dateutil.DayStart(now).AddDate(0, 0, 1),
	}
}

// Change is one window's net-worth movement: both ends, the window that
// produced them, and the currency they are expressed in. It carries its bounds
// so a view can never print the delta without being able to name the window.
type Change struct {
	Window
	// Base is the currency both ends are expressed in.
	Base string
	// StartMinor / EndMinor are net worth as of Since and Until, in base minor
	// units. Liabilities subtract, archived accounts are excluded, exactly as in
	// ledger.NetWorth.
	StartMinor, EndMinor int64
	// Known is false when the window could not be read (no accounts, or an FX
	// rate was missing); a view must show nothing rather than a confident zero.
	Known bool
}

// DeltaMinor is the signed movement across the window.
func (c Change) DeltaMinor() int64 { return c.EndMinor - c.StartMinor }

// Delta is DeltaMinor as money in the base currency.
func (c Change) Delta() money.Money { return money.New(c.DeltaMinor(), c.Base) }

// PercentChange is the movement as a whole percentage of where the window
// opened, and whether that percentage is meaningful at all (it is not when net
// worth opened at zero, and it is misleading when it opened negative).
func (c Change) PercentChange() (int64, bool) {
	if !c.Known {
		return 0, false
	}
	return ledger.PercentChange(c.EndMinor, c.StartMinor)
}

// Flat reports whether nothing moved.
func (c Change) Flat() bool { return c.Known && c.DeltaMinor() == 0 }

// Compute reads net worth at both ends of the window for the given accounts.
//
// Callers pass whatever account set their surface is scoped to; the scope is
// the caller's decision, the arithmetic is not. An error from the ledger (a
// missing FX rate, a currency mismatch) yields a Change with Known false rather
// than a zero delta that reads as "nothing happened".
func Compute(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, w Window) (Change, error) {
	c := Change{Window: w, Base: rates.Base}
	series, err := ledger.NetWorthSeries(accounts, txns, []time.Time{w.Since, w.Until}, rates)
	if err != nil {
		return c, fmt.Errorf("nwchange: window %s → %s: %w",
			dateutil.FormatDate(w.Since), dateutil.FormatDate(w.Until), err)
	}
	if len(series) != 2 {
		return c, fmt.Errorf("nwchange: expected 2 net-worth points, got %d", len(series))
	}
	c.StartMinor, c.EndMinor = series[0].Amount, series[1].Amount
	c.Known = true
	return c, nil
}

// MonthToDateChange is the shorthand every "this month" figure in the app uses.
func MonthToDateChange(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, now time.Time) (Change, error) {
	return Compute(accounts, txns, rates, MonthToDate(now))
}
