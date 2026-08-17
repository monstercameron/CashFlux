// SPDX-License-Identifier: MIT

// Package clearwatch finds charges that have not cleared when this account's own
// charges normally would have (EC-4).
//
// # The window has to be the account's, not a number I picked
//
// A debit card settles overnight and a credit card takes a few days, so a single
// "flag anything over 5 days" rule is wrong for both: it nags about every card
// charge and stays quiet about a debit that vanished. The window is learned from
// the account's own history — how long its charges have actually taken — and an
// account without enough history gets no window and no flags.
//
// # A window learned from three transactions is not a window
//
// The whole output is "this is unusual for you", which is a claim about a normal
// that has to exist first. Below MinSamples the package reports that it does not
// know, and reporting nothing is right: an account somebody started using last
// week has no normal to be late against.
//
// The median is used rather than the mean because one charge that took six weeks
// to post — a dispute, a hold, a correction — would drag a mean far enough that
// nothing ever looks late again.
package clearwatch

import "sort"

// MinSamples is how many cleared charges an account needs before its typical
// window means anything.
//
// Eight. Fewer and the median moves a full day with each new observation, so the
// window somebody is being judged against would change every time they cleared
// something.
const MinSamples = 8

// GraceDays is how far past the typical window a charge goes before it is worth
// mentioning.
//
// Three days. Half the reason a charge is late is a weekend or a bank holiday,
// and flagging on the first day past the median would fire on most Fridays.
const GraceDays = 3

// MaxWindowDays caps a learned window.
//
// Thirty days. An account whose median is longer than a month is not telling us
// about clearing time — it is telling us its entries are being entered late, or
// reconciled in batches — and a "late" flag against a 60-day normal would be
// meaningless in both directions.
const MaxWindowDays = 30

// Window is what an account's history says about how long its charges take.
type Window struct {
	// Known is false when the account has too little history. Reported rather
	// than a window of zero, because zero would flag every charge on the day it
	// happened.
	Known bool
	// TypicalDays is the median observed clearing time.
	TypicalDays int
	// Samples is how many cleared charges it was learned from — the evidence
	// behind any flag raised against it.
	Samples int
}

// Learn works out an account's typical clearing time from the days its charges
// have actually taken. Callers pass only known values (see
// domain.Transaction.DaysToClear), so a row with no recorded moment cannot be
// mistaken for one that cleared instantly.
func Learn(observed []int) Window {
	clean := make([]int, 0, len(observed))
	for _, d := range observed {
		if d >= 0 {
			clean = append(clean, d)
		}
	}
	if len(clean) < MinSamples {
		return Window{}
	}
	sort.Ints(clean)
	med := clean[len(clean)/2]
	if len(clean)%2 == 0 {
		med = (clean[len(clean)/2-1] + clean[len(clean)/2]) / 2
	}
	if med > MaxWindowDays {
		return Window{}
	}
	return Window{Known: true, TypicalDays: med, Samples: len(clean)}
}

// OverdueBy reports how many days past this account's normal an uncleared charge
// has gone, and whether it is worth mentioning at all.
//
// ageDays is how long ago the charge happened. A window that is not Known yields
// no verdict: an account with no normal cannot have something abnormal.
func (w Window) OverdueBy(ageDays int) (days int, overdue bool) {
	if !w.Known || ageDays < 0 {
		return 0, false
	}
	limit := w.TypicalDays + GraceDays
	if ageDays <= limit {
		return 0, false
	}
	return ageDays - limit, true
}
