// SPDX-License-Identifier: MIT

package budgeting

import "math"

// FundingRead separates two things a zero-based plan constantly conflates: how
// much income the plan is BUILT on, and how much has actually arrived (C587).
//
// The budgets page could show 100% of an expected $10,709.16 assigned while only
// $6,961.00 had been received, with the $3,748.16 difference reported nowhere
// except a separate month-close screen. "Fully assigned" and "fully funded" then
// look identical, and the one that matters for whether a payment clears is the
// one that was missing.
type FundingRead struct {
	// Expected is the income basis the plan was built against.
	Expected int64
	// Received is what has actually arrived so far this period.
	Received int64
	// Assigned is the total handed out to budgets and savings.
	Assigned int64
	// Fixed is the part of Assigned that a bulk budget adjustment cannot scale —
	// today, the monthly amounts assigned to savings and investment accounts.
	//
	// It exists because the reconcile action hands its percentage to the budget
	// adjuster, which only ever touches budgets. Computing the reduction against
	// the WHOLE assigned figure produced a percentage that left the plan still
	// short whenever a household saved anything: assign $1,000 to budgets and $500
	// to savings against $1,000 received, and a −33.3%% cut across budgets leaves
	// $667 + $500 = $1,167 — still $167 over. The button claimed to resolve the
	// gap and did not.
	Fixed int64
}

// Adjustable is the part of the plan a bulk budget adjustment can actually move.
func (f FundingRead) Adjustable() int64 {
	if adj := f.Assigned - f.Fixed; adj > 0 {
		return adj
	}
	return 0
}

// Unfunded is the part of the plan that no arrived money backs yet. Zero when
// enough has come in.
func (f FundingRead) Unfunded() int64 {
	if gap := f.Assigned - f.Received; gap > 0 {
		return gap
	}
	return 0
}

// FullyFunded reports whether everything assigned is backed by money in hand.
func (f FundingRead) FullyFunded() bool { return f.Unfunded() == 0 }

// Shortfall is how far the income that arrived falls behind what the plan
// expected — a different question from Unfunded, and the one that says whether
// the EXPECTATION was wrong rather than whether the assignments were.
func (f FundingRead) Shortfall() int64 {
	if gap := f.Expected - f.Received; gap > 0 {
		return gap
	}
	return 0
}

// Meaningful reports whether there is enough information to say anything. With
// no expectation and nothing received, the read is silent rather than claiming a
// household with no income data is unfunded.
func (f FundingRead) Meaningful() bool { return f.Expected > 0 || f.Received > 0 }

// ReceivedPct is how far along the track the arrived money reaches, as a percent
// of whichever is larger — the plan or the expectation — so the marker sits on
// the same scale the allocation bar already uses.
func (f FundingRead) ReceivedPct() int {
	scale := f.Expected
	if f.Assigned > scale {
		scale = f.Assigned
	}
	if scale <= 0 {
		return 0
	}
	if p := int(f.Received * 100 / scale); p < 100 {
		return p
	}
	return 100
}

// ReduceToFitPct is the percentage change that would scale every assignment down
// to the money actually received — the number the "bring the plan down to what
// has arrived" action hands to the bulk adjuster.
//
// It is negative (a reduction), rounded to one decimal place so the control it
// feeds shows a number a person would type, and clamped to the adjuster's own
// bounds. It returns 0 when nothing needs reducing, when there is nothing
// assigned to scale, or when the required cut is deeper than a single bulk
// adjustment can express — in which case the UI must not offer a one-click fix
// that would quietly do something else.
func (f FundingRead) ReduceToFitPct() float64 {
	adj := f.Adjustable()
	if f.FullyFunded() || adj <= 0 || f.Received < 0 {
		return 0
	}
	// The budgets have to come down to whatever the received money leaves once the
	// unscalable part is honoured. If that is nothing, no percentage exists that
	// closes the gap — zeroing every budget would not do it — and offering a
	// one-click fix that quietly did something else is worse than offering none.
	target := f.Received - f.Fixed
	if target <= 0 {
		return 0
	}
	raw := (float64(target)/float64(adj) - 1) * 100
	pct := math.Round(raw*10) / 10
	if pct >= 0 || pct < AdjustMinPct {
		return 0
	}
	return pct
}
