// SPDX-License-Identifier: MIT

// Package fundtradeoff says what funding one goal now costs the others
// (EC-12): "putting $500 into the car fund delays the vacation by about two
// months".
//
// # A trade-off is only real when the money is contested
//
// If the pot covers everything, funding one goal delays nothing, and saying it
// does is scaremongering that teaches people to ignore the warning. This
// reports a cost ONLY when the money taken actually comes out of something
// else — which is the difference between a decision aid and a guilt machine.
//
// # It names the goal that pays
//
// "This will slow your other goals" is unfalsifiable and unactionable. The
// reader needs to know WHICH one and by how long, because the answer decides
// whether they mind: two months on a holiday they have not booked is different
// from two months on a deposit with a date attached.
package fundtradeoff

import "sort"

// Goal is one destination competing for the same monthly money.
type Goal struct {
	ID, Name string
	// RemainingMinor is what is still needed.
	RemainingMinor int64
	// MonthlyMinor is what it currently receives each month. Zero means it is not
	// being funded, so there is no date to delay — a goal nobody is contributing
	// to cannot be made later.
	MonthlyMinor int64
}

// Cost is what one goal pays for another being funded first.
type Cost struct {
	GoalID, GoalName string
	// MonthsDelayed is how much later it lands. Zero with Stalls=true means it
	// stops landing at all.
	MonthsDelayed int
	// Stalls says the goal loses so much that it no longer completes on any
	// horizon worth stating. "Delayed by 9,999 months" is not information; "this
	// stops it entirely" is.
	Stalls bool
	// LostMonthlyMinor is what this goal gives up each month, so the reader can
	// check the arithmetic rather than trust it.
	LostMonthlyMinor int64
}

// Assess works out who pays for taking takingMinor a month, and how much later
// they land.
//
// takingMinor is the monthly amount being diverted; slackMinor is money in the
// pot that nothing else is claiming. The slack is spent FIRST — it is not a
// trade-off to spend money nobody wanted — and only what is left comes out of
// the goals, lowest priority first, matching how a funding waterfall actually
// cascades.
//
// lower is the goals that would lose out, in the order they would lose it.
func Assess(takingMinor, slackMinor int64, lower []Goal) []Cost {
	if takingMinor <= 0 {
		return nil
	}
	need := takingMinor - slackMinor
	if need <= 0 {
		return nil // the pot covered it; nothing else pays
	}

	var out []Cost
	for _, g := range lower {
		if need <= 0 {
			break
		}
		if g.MonthlyMinor <= 0 || g.RemainingMinor <= 0 {
			continue // nothing to take, or nothing left to delay
		}
		lost := g.MonthlyMinor
		if lost > need {
			lost = need
		}
		need -= lost

		before := ceilDiv(g.RemainingMinor, g.MonthlyMinor)
		after := g.MonthlyMinor - lost
		if after <= 0 {
			out = append(out, Cost{GoalID: g.ID, GoalName: g.Name, Stalls: true, LostMonthlyMinor: lost})
			continue
		}
		delay := int(ceilDiv(g.RemainingMinor, after) - before)
		if delay <= 0 {
			continue // it loses money but lands in the same month; not worth saying
		}
		out = append(out, Cost{GoalID: g.ID, GoalName: g.Name, MonthsDelayed: delay, LostMonthlyMinor: lost})
	}
	// Worst first: the reader can stop as soon as they have seen the thing that
	// would put them off, which is the same ordering rule the action preview uses.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Stalls != out[j].Stalls {
			return out[i].Stalls
		}
		return out[i].MonthsDelayed > out[j].MonthsDelayed
	})
	return out
}

// Any reports whether anything actually pays.
func Any(costs []Cost) bool { return len(costs) > 0 }

// ceilDiv rounds up: a goal is not reached until the last partial month has
// actually happened.
func ceilDiv(a, b int64) int64 {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
