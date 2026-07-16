// SPDX-License-Identifier: MIT

// Package merchantstats derives a merchant's spending "story" from its charge
// history — the numbers behind the transaction-detail context panel (TX6):
// typical amount, how a given charge compares, how often you've visited this week
// and month, this month's total versus a typical month, and the last-twelve-
// charge series for a sparkline.
//
// It is pure (no syscall/js, stdlib + dateutil only), so it unit-tests on native
// Go. Callers resolve the merchant name (via the TX1 payee-alias resolver) and
// convert amounts to base-currency minor units before handing charges in, so this
// package deals only in clean magnitudes and dates.
package merchantstats

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// MinCharges is the number of charges a merchant needs before the context panel
// is worth showing: below this the "typical" and "visit" figures are noise, so
// the caller omits the panel for one-off merchants.
const MinCharges = 3

// last12Window is how many of the most recent charges feed the "typical amount"
// median and the sparkline series.
const last12Window = 12

// Charge is one expense at the merchant: a positive magnitude in base-currency
// minor units and the date it posted. Transfers and income are excluded by the
// caller.
type Charge struct {
	Date  time.Time
	Minor int64 // magnitude (positive), base-currency minor units
}

// Stats is the computed merchant story. All money figures are base-currency minor
// units. Enough reports whether there were at least MinCharges charges — when
// false the caller should omit the panel.
type Stats struct {
	Count           int
	Enough          bool
	TypicalMinor    int64   // median of the last 12 charges
	VisitsThisWeek  int     // charges since the start of the current week
	VisitsThisMonth int     // charges in the current calendar month
	SpentThisMonth  int64   // total spent this calendar month
	TypicalMonth    int64   // median of prior complete months' totals
	Last12          []int64 // up to 12 most recent magnitudes, oldest → newest
	// Last12Dates are the charge dates parallel to Last12 (same length/order), so a
	// sparkline can label its time span and mark which point is which charge.
	Last12Dates []time.Time
}

// Compute derives the merchant story from its charges as of now. Charges may be
// in any order; Compute sorts a copy. weekStart is the household's first weekday.
func Compute(charges []Charge, now time.Time, weekStart time.Weekday) Stats {
	s := Stats{Count: len(charges)}
	if len(charges) == 0 {
		return s
	}
	s.Enough = len(charges) >= MinCharges

	sorted := make([]Charge, len(charges))
	copy(sorted, charges)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })

	// Last-12 series (oldest → newest) and its median = typical amount.
	from := 0
	if len(sorted) > last12Window {
		from = len(sorted) - last12Window
	}
	recent := sorted[from:]
	s.Last12 = make([]int64, len(recent))
	s.Last12Dates = make([]time.Time, len(recent))
	mags := make([]int64, len(recent))
	for i, c := range recent {
		s.Last12[i] = c.Minor
		s.Last12Dates[i] = c.Date
		mags[i] = c.Minor
	}
	s.TypicalMinor = median(mags)

	// This-week and this-month visit counts + spend.
	weekFrom := dateutil.WeekStart(now, weekStart)
	monthFrom := dateutil.MonthStart(now)
	byMonth := map[string]int64{}
	curKey := monthFrom.Format("2006-01")
	for _, c := range sorted {
		if !c.Date.Before(weekFrom) && !c.Date.After(now) {
			s.VisitsThisWeek++
		}
		if !c.Date.Before(monthFrom) && !c.Date.After(now) {
			s.VisitsThisMonth++
			s.SpentThisMonth += c.Minor
		}
		byMonth[c.Date.Format("2006-01")] += c.Minor
	}

	// Typical month = median of prior (complete) months' totals, excluding the
	// current partial month so a mid-month figure isn't compared to full months.
	var priorTotals []int64
	for k, v := range byMonth {
		if k == curKey {
			continue
		}
		priorTotals = append(priorTotals, v)
	}
	sort.Slice(priorTotals, func(i, j int) bool { return priorTotals[i] < priorTotals[j] })
	s.TypicalMonth = median(priorTotals)
	return s
}

// DeltaVsTypical returns how much a charge of txnMinor (magnitude, base minor)
// runs above (+) or below (−) the merchant's typical amount.
func (s Stats) DeltaVsTypical(txnMinor int64) int64 { return txnMinor - s.TypicalMinor }

// median returns the integer median of xs (which need not be sorted). Empty → 0.
func median(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	c := make([]int64, len(xs))
	copy(c, xs)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}
