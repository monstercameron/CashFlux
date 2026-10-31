// SPDX-License-Identifier: MIT

// Package acctproject projects a single account's balance forward over a short
// horizon (AC13 — "Checking $2,340 today → ~$1,150 low on the 28th"). It walks the
// dated cash-flow drivers scoped to one account — recurring occurrences and
// scheduled bills the caller has already resolved — and reports the running low
// point plus the driver list behind it, so the row can show the number AND explain
// it ("rent −$1,400 on the 1st"). Pure Go, no syscall/js: amounts are signed
// integer minor units (inflow positive, outflow negative) and it unit-tests on
// native Go. The forecast/billsched packages solve the household-wide curve; this
// is the deliberately simple per-account slice.
package acctproject

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrCurrencyMismatch is money.ErrCurrencyMismatch, kept so callers that match a
// projection's currency failure against this package keep working.
var ErrCurrencyMismatch = money.ErrCurrencyMismatch

// Driver is one dated cash flow scoped to the account: a recurring paycheck, a
// bill, a scheduled transfer. Amount is signed minor units (positive = money in,
// negative = money out).
type Driver struct {
	Label  string
	Date   time.Time
	Amount int64
}

// Projection is the account's projected curve over the horizon: the starting
// balance, the drivers applied (date-sorted), the projected low point and the date
// it occurs, and the balance at the end of the horizon.
type Projection struct {
	Start   int64
	Drivers []Driver
	Low     int64
	LowDate time.Time
	End     int64
}

// HasLowDip reports whether the projected low is strictly below the starting
// balance — i.e. whether the "→ ~$X low on the Nth" clause is worth showing at all.
// A flat or rising account has nothing to warn about.
func (p Projection) HasLowDip() bool { return p.Low < p.Start }

// Project computes the account's balance curve from `start` (today's balance, minor
// units) over [asOf, asOf+horizonDays], applying every driver that falls inside the
// window in date order. Drivers before asOf are treated as landing today; drivers
// past the horizon are dropped. The returned Drivers slice is sorted by date (ties
// by label) and contains only the in-window drivers actually applied. All of a
// day's drivers are applied before the balance is compared, so Low, LowDate and
// FirstNegative describe closing balances and never depend on the label order of
// same-day drivers. A non-positive horizon yields a flat projection at `start`.
func Project(start int64, asOf time.Time, drivers []Driver, horizonDays int) (Projection, error) {
	asOf = day(asOf)
	p := Projection{Start: start, Low: start, LowDate: asOf, End: start}
	if horizonDays <= 0 {
		return p, nil
	}
	end := asOf.AddDate(0, 0, horizonDays)

	inWindow := make([]Driver, 0, len(drivers))
	for _, d := range drivers {
		dd := day(d.Date)
		if dd.Before(asOf) {
			dd = asOf // an already-due flow lands today
		}
		if dd.After(end) {
			continue
		}
		inWindow = append(inWindow, Driver{Label: d.Label, Date: dd, Amount: d.Amount})
	}
	sort.SliceStable(inWindow, func(i, j int) bool {
		if !inWindow[i].Date.Equal(inWindow[j].Date) {
			return inWindow[i].Date.Before(inWindow[j].Date)
		}
		return inWindow[i].Label < inWindow[j].Label
	})

	bal, err := walkDayCloses(start, inWindow, func(date time.Time, closing int64) bool {
		if closing < p.Low {
			p.Low, p.LowDate = closing, date
		}
		return true
	})
	if err != nil {
		return Projection{}, err
	}
	p.Drivers = inWindow
	p.End = bal
	return p, nil
}

// walkDayCloses applies date-sorted drivers to start one calendar day at a time and
// calls visit with each day's date and closing balance, in order, until visit
// returns false. It returns the balance after the last day visited.
func walkDayCloses(start int64, drivers []Driver, visit func(date time.Time, closing int64) bool) (int64, error) {
	bal := start
	for i := 0; i < len(drivers); {
		date := day(drivers[i].Date)
		for ; i < len(drivers) && day(drivers[i].Date).Equal(date); i++ {
			var err error
			bal, err = money.Add(bal, drivers[i].Amount)
			if err != nil {
				return 0, fmt.Errorf("acctproject: apply %q on %s: %w", drivers[i].Label, date.Format("2006-01-02"), err)
			}
		}
		if !visit(date, bal) {
			break
		}
	}
	return bal, nil
}

// day canonicalizes a time to the calendar date it shows in its own location,
// at 00:00 UTC (dateutil.DayStart), so comparisons across the mixed locations the
// caller feeds in ("same day" vs "before") stay honest.
func day(t time.Time) time.Time { return dateutil.DayStart(t) }

// ─── C381: the curve, not just its low point ─────────────────────────────────

// Point is one day of the projected curve: the closing balance and what moved
// that day (Out as a positive magnitude, so a caller need not re-derive signs).
type Point struct {
	Date    time.Time
	Balance int64
	In, Out int64
}

// Series expands the projection into one point per day over the horizon: day zero
// through day horizonDays, horizonDays+1 points with strictly increasing dates.
//
// Project alone answers "how low does it get", which is the right answer for a
// one-line warning on a row. Drawing the forward line under the balance chart
// needs the shape, and the shape has to be DAILY: an account that ends every
// month comfortably positive and dips below zero on the 23rd looks perfectly
// healthy at month granularity, and the dip is the entire point of showing it.
//
// Every point is a closing balance. The day-zero point is today's balance after
// whatever lands today, so it equals Start unless a driver (or an overdue flow
// Project moved to today) is dated asOf. Only the drivers Project kept are drawn,
// and only those dated within [day(asOf), day(asOf)+horizonDays]: a Series asked
// for a later asOf than the Project call ignores the earlier drivers rather than
// folding them into the opening balance. A non-positive horizon yields nothing.
func (p Projection) Series(asOf time.Time, horizonDays int) ([]Point, error) {
	if horizonDays <= 0 {
		return nil, nil
	}
	start := day(asOf)
	byDay := map[string][]Driver{}
	for _, d := range p.Drivers {
		k := day(d.Date).Format("2006-01-02")
		byDay[k] = append(byDay[k], d)
	}
	out := make([]Point, 0, horizonDays+1)
	bal := p.Start
	for i := 0; i <= horizonDays; i++ {
		dt := start.AddDate(0, 0, i)
		in, outAmt, next, err := applyDay(bal, byDay[dt.Format("2006-01-02")])
		if err != nil {
			return nil, err
		}
		bal = next
		out = append(out, Point{Date: dt, Balance: bal, In: in, Out: outAmt})
	}
	return out, nil
}

func applyDay(balance int64, drivers []Driver) (in, out, nextBalance int64, err error) {
	nextBalance = balance
	for _, driver := range drivers {
		nextBalance, err = money.Add(nextBalance, driver.Amount)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("acctproject: apply %q on %s: %w", driver.Label, driver.Date.Format("2006-01-02"), err)
		}
		if driver.Amount > 0 {
			in, err = money.Add(in, driver.Amount)
		} else {
			var magnitude int64
			magnitude, err = money.Abs(driver.Amount)
			if err == nil {
				out, err = money.Add(out, magnitude)
			}
		}
		if err != nil {
			return 0, 0, 0, fmt.Errorf("acctproject: summarize daily flows on %s: %w", driver.Date.Format("2006-01-02"), err)
		}
	}
	return in, out, nextBalance, nil
}

// FirstNegative reports the first day the projected balance closes below zero,
// and whether it happens at all.
//
// A day counts when its closing balance, after every driver dated that day, is
// below zero; an intra-day dip that recovers by close does not. Drivers must be
// date-sorted, as Project returns them.
//
// FIRST, not lowest. Knowing when trouble starts is what leaves time to act; the
// deepest point is usually later than the point where the decision had to be
// made. A start balance already below zero counts as today — otherwise an
// overdrawn account projects as "never goes negative", which is the most
// misleading answer available.
func (p Projection) FirstNegative(asOf time.Time) (time.Time, bool, error) {
	start := day(asOf)
	if p.Start < 0 {
		return start, true, nil
	}
	var negativeOn time.Time
	found := false
	_, err := walkDayCloses(p.Start, p.Drivers, func(date time.Time, closing int64) bool {
		if closing < 0 {
			negativeOn, found = date, true
		}
		return !found
	})
	if err != nil {
		return time.Time{}, false, err
	}
	return negativeOn, found, nil
}

// Known reports whether the projection rests on anything. A horizon with no
// drivers is not a forecast of a flat balance — it is the absence of
// information, and drawing a confident flat line would claim knowledge the app
// does not have.
func (p Projection) Known() bool { return len(p.Drivers) > 0 }
