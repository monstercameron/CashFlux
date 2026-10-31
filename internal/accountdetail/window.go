// SPDX-License-Identifier: MIT

// Package accountdetail holds the pure decisions behind an account's detail
// panel, kept out of view code so they unit-test on native Go.
package accountdetail

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/acctseries"
	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// Range keys name the balance chart's selectable windows.
const (
	Range90d = "90d"
	Range12m = "12m"
	RangeAll = "all"
)

// allWindowCapDays bounds the "all" window at twenty years of daily points.
const allWindowCapDays = 366 * 20

// Window is the balance-chart window chosen for one account.
type Window struct {
	// Days is the number of daily points to draw, ending at the as-of day.
	Days int
	// HasRange is set when the account has more than 90 days of history, so the
	// chart offers the 90d / 12m / all picker.
	HasRange bool
}

// SelectWindow picks the chart window for the selected range key. earliest is the
// date of the account's first transaction (zero when it has none). An account
// without more than 90 days of history always gets the 90-day window, whatever
// range was selected, and an unknown key selects 90 days. Both dates count as the
// calendar day they show in their own location, so a clock with a UTC offset does
// not add a day to the span.
func SelectWindow(selected string, asOf, earliest time.Time) Window {
	asOf = dateutil.DayStart(asOf)
	if !earliest.IsZero() {
		earliest = dateutil.DayStart(earliest)
	}
	allDays := acctseries.AllDays(asOf, acctseries.Days90, allWindowCapDays, earliest)
	w := Window{Days: acctseries.Days90, HasRange: acctseries.HasRange(allDays)}
	if !w.HasRange {
		return w
	}
	switch selected {
	case Range12m:
		w.Days = acctseries.Days12m
	case RangeAll:
		w.Days = allDays
	}
	return w
}
