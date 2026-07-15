// SPDX-License-Identifier: MIT

// Package agemoney computes the "Age of Money" metric: how long, on average, a
// dollar sits between when it was earned and when it is spent. A higher age means
// you are spending money you earned longer ago — a buffer, rather than living
// paycheck-to-paycheck.
//
// The metric is computed FIFO, the way YNAB does it: each spent dollar is matched
// to the OLDEST unspent income dollar, and the "age" of that spend is the number
// of days between the income date and the spend date. The reported figure is the
// dollar-weighted average age of the most recent window of outflows.
//
// This package is pure — no syscall/js, no money formatting, no currency lookups.
// The caller converts every flow to a single base currency (minor units) and
// excludes transfers before calling Compute; the result is explained by its
// breakdown fields so the number is never a black box.
package agemoney

import "time"

// DefaultWindow is the number of most-recent outflow transactions averaged into
// the reported age when Opts.Window is unset. It mirrors YNAB's rolling window of
// roughly the last ten outflows, which keeps the figure responsive to how you are
// spending right now rather than your entire history.
const DefaultWindow = 10

// Flow is one money movement in the base currency, minor units. A positive
// AmountMinor is income (money earned), a negative AmountMinor is an expense
// (money spent). A zero amount is ignored. Flows must be passed in time order
// (oldest first); the caller sorts and excludes transfers.
type Flow struct {
	Date        time.Time
	AmountMinor int64
}

// DefaultMaxAgeDays caps how old a single spent dollar can count as. Beyond about a
// year of buffer the exact age stops being a useful signal — "you're well ahead" is
// the message, whether it's 400 or 900 days — and an uncapped figure balloons to
// absurd values for anyone with a long-standing surplus (their oldest unspent income
// is years old). Capping keeps the metric meaningful and honest for everyone.
const DefaultMaxAgeDays = 365

// Opts configures the computation.
type Opts struct {
	// Window is how many of the most recent outflow transactions to average over.
	// Values <= 0 fall back to DefaultWindow.
	Window int
	// MaxAgeDays clamps each spent dollar's age. Values <= 0 fall back to
	// DefaultMaxAgeDays.
	MaxAgeDays int
}

// Result is the age-of-money figure plus the breakdown that explains it.
type Result struct {
	// Days is the dollar-weighted average age, in whole days, of the outflows in
	// the window. Meaningful only when Ready is true.
	Days int
	// Ready reports whether there is enough matched history to trust Days. It is
	// false when there are no outflows, when no outflow could be matched to prior
	// income, or when an outflow in the window spent more than the tracked income
	// could cover (a sign the ledger's income history is incomplete).
	Ready bool
	// TotalAgedMinor is the sum of matched spend (minor units) across the window —
	// the denominator of the weighted average.
	TotalAgedMinor int64
	// WindowStart and WindowEnd are the dates of the oldest and newest outflow in
	// the window, so the figure can say what span it covers.
	WindowStart time.Time
	WindowEnd   time.Time
	// WindowCount is how many outflows fell inside the window (<= Opts.Window).
	WindowCount int
	// Capped reports that the figure hit the MaxAgeDays ceiling — i.e. the buffer is
	// at least that old, so the UI should read "365+ days" rather than an exact number.
	Capped bool
}

const day = 24 * time.Hour

// lot is a parcel of still-unspent income sitting in the FIFO queue.
type lot struct {
	date      time.Time
	remaining int64
}

// spend is the aged record of one outflow: how much of it was matched to income,
// the dollar-days that match accrued, and whether any of it went unmatched.
type spend struct {
	date      time.Time
	matched   int64
	weighted  int64 // Σ(portionMinor × ageDays)
	unmatched bool
}

// Compute returns the age of money for the given time-ordered flows. It walks the
// flows once, building a FIFO queue of income lots and, for each expense,
// consuming from the oldest lots to age every spent dollar. The reported Days is
// the dollar-weighted average age over the trailing window of outflows.
func Compute(flows []Flow, opts Opts) Result {
	window := opts.Window
	if window <= 0 {
		window = DefaultWindow
	}
	maxAge := opts.MaxAgeDays
	if maxAge <= 0 {
		maxAge = DefaultMaxAgeDays
	}

	var lots []lot
	head := 0 // index of the oldest lot with remaining balance
	var spends []spend

	for _, f := range flows {
		switch {
		case f.AmountMinor > 0:
			lots = append(lots, lot{date: f.Date, remaining: f.AmountMinor})
		case f.AmountMinor < 0:
			need := -f.AmountMinor
			sp := spend{date: f.Date}
			for need > 0 && head < len(lots) {
				l := &lots[head]
				take := l.remaining
				if take > need {
					take = need
				}
				age := int64(f.Date.Sub(l.date) / day)
				if age < 0 {
					age = 0
				}
				if age > int64(maxAge) {
					age = int64(maxAge)
				}
				sp.matched += take
				sp.weighted += take * age
				l.remaining -= take
				need -= take
				if l.remaining == 0 {
					head++
				}
			}
			if need > 0 {
				sp.unmatched = true
			}
			spends = append(spends, sp)
		}
	}

	if len(spends) == 0 {
		return Result{Ready: false}
	}

	start := len(spends) - window
	if start < 0 {
		start = 0
	}
	win := spends[start:]

	var totMatched, totWeighted int64
	ready := true
	for _, s := range win {
		if s.unmatched {
			ready = false
		}
		totMatched += s.matched
		totWeighted += s.weighted
	}

	res := Result{
		WindowCount:    len(win),
		WindowStart:    win[0].date,
		WindowEnd:      win[len(win)-1].date,
		TotalAgedMinor: totMatched,
	}
	if !ready || totMatched == 0 {
		res.Ready = false
		return res
	}
	res.Ready = true
	// Round to the nearest whole day: (weighted + matched/2) / matched.
	res.Days = int((totWeighted + totMatched/2) / totMatched)
	if res.Days >= maxAge {
		res.Days = maxAge
		res.Capped = true
	}
	return res
}
