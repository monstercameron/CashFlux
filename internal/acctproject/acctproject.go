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
	"sort"
	"time"
)

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
// by label) and contains only the in-window drivers actually applied. A non-positive
// horizon yields a flat projection at `start`.
func Project(start int64, asOf time.Time, drivers []Driver, horizonDays int) Projection {
	asOf = day(asOf)
	p := Projection{Start: start, Low: start, LowDate: asOf, End: start}
	if horizonDays <= 0 {
		return p
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

	bal := start
	for _, d := range inWindow {
		bal += d.Amount
		if bal < p.Low {
			p.Low, p.LowDate = bal, d.Date
		}
	}
	p.Drivers = inWindow
	p.End = bal
	return p
}

// day canonicalizes a time to its UTC calendar day so comparisons across the mixed
// locations the caller feeds in ("same day" vs "before") stay honest.
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ─── C381: the curve, not just its low point ─────────────────────────────────

// Point is one day of the projected curve: the closing balance and what moved
// that day (Out as a positive magnitude, so a caller need not re-derive signs).
type Point struct {
	Date    time.Time
	Balance int64
	In, Out int64
}

// Series expands the projection into one point per day over the horizon,
// starting with the untouched balance today.
//
// Project alone answers "how low does it get", which is the right answer for a
// one-line warning on a row. Drawing the forward line under the balance chart
// needs the shape, and the shape has to be DAILY: an account that ends every
// month comfortably positive and dips below zero on the 23rd looks perfectly
// healthy at month granularity, and the dip is the entire point of showing it.
//
// The first point is today's balance with nothing applied, so the drawn line
// starts where the account actually is and cannot contradict the figure printed
// beside it. A non-positive horizon yields nothing.
func (p Projection) Series(asOf time.Time, horizonDays int) []Point {
	if horizonDays <= 0 {
		return nil
	}
	start := day(asOf)
	byDay := map[string][]Driver{}
	for _, d := range p.Drivers {
		k := day(d.Date).Format("2006-01-02")
		byDay[k] = append(byDay[k], d)
	}
	out := make([]Point, 0, horizonDays+1)
	out = append(out, Point{Date: start, Balance: p.Start})
	bal := p.Start
	for i := 1; i <= horizonDays; i++ {
		dt := start.AddDate(0, 0, i)
		var in, outAmt int64
		for _, d := range byDay[dt.Format("2006-01-02")] {
			bal += d.Amount
			if d.Amount > 0 {
				in += d.Amount
			} else {
				outAmt += -d.Amount
			}
		}
		out = append(out, Point{Date: dt, Balance: bal, In: in, Out: outAmt})
	}
	return out
}

// FirstNegative reports the first day the projected balance closes below zero,
// and whether it happens at all.
//
// FIRST, not lowest. Knowing when trouble starts is what leaves time to act; the
// deepest point is usually later than the point where the decision had to be
// made. A start balance already below zero counts as today — otherwise an
// overdrawn account projects as "never goes negative", which is the most
// misleading answer available.
func (p Projection) FirstNegative(asOf time.Time) (time.Time, bool) {
	start := day(asOf)
	if p.Start < 0 {
		return start, true
	}
	bal := p.Start
	for _, d := range p.Drivers {
		bal += d.Amount
		if bal < 0 {
			return day(d.Date), true
		}
	}
	return time.Time{}, false
}

// Known reports whether the projection rests on anything. A horizon with no
// drivers is not a forecast of a flat balance — it is the absence of
// information, and drawing a confident flat line would claim knowledge the app
// does not have.
func (p Projection) Known() bool { return len(p.Drivers) > 0 }
