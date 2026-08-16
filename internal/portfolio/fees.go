// SPDX-License-Identifier: MIT

package portfolio

import "math"

// ─── C378: what the funds cost to hold ───────────────────────────────────────
//
// An expense ratio is the one portfolio cost that never appears as a
// transaction: it is deducted inside the fund, so the ledger never sees it and
// no amount of categorising will surface it. A household can hold an expensive
// fund for a decade without a single row anywhere saying so.
//
// The arithmetic is deliberately the simple one — current value × ratio — rather
// than a time-weighted average of balances. It answers "at what I hold today,
// what is this costing me a year", which is the question that changes a
// decision; a more precise historical figure would need a price history the app
// does not keep, and would be a worse answer to a question nobody asked.

// FeeDrag is the annual cost of the expense ratios across a set of holdings.
type FeeDrag struct {
	// AnnualMinor is the yearly cost in minor units at current market value.
	AnnualMinor int64
	// WeightedBps is the portfolio-wide average expense ratio, weighted by value
	// — the single number that says "you are paying about this much".
	WeightedBps float64
	// CoveredMinor is the market value of holdings that HAVE a ratio recorded,
	// and TotalMinor the value of all of them. A weighted average over a third of
	// the portfolio is not a portfolio-wide figure, and a view that shows one
	// without the other invites reading it as though it were.
	CoveredMinor, TotalMinor int64
}

// Known reports whether any holding carries an expense ratio at all. With none,
// every field is zero and the surface should say nothing rather than claim the
// portfolio is free.
func (f FeeDrag) Known() bool { return f.CoveredMinor > 0 }

// CoveragePct is the share of portfolio value the figure actually speaks for.
func (f FeeDrag) CoveragePct() float64 {
	if f.TotalMinor == 0 {
		return 0
	}
	return float64(f.CoveredMinor) / float64(f.TotalMinor) * 100
}

// Complete reports whether every holding with value carries a ratio, so a view
// can present the figure plainly instead of qualifying it.
func (f FeeDrag) Complete() bool { return f.TotalMinor > 0 && f.CoveredMinor == f.TotalMinor }

// Fees computes the annual expense-ratio cost across holdings.
//
// Only holdings with a positive ratio contribute to the cost AND to the
// weighting: a fund with no ratio recorded is unknown, not free, and averaging
// it in as zero would quietly understate the number the household is trying to
// judge.
func Fees(hs []Holding) FeeDrag {
	var out FeeDrag
	var weighted float64
	for _, h := range hs {
		v := HoldingValueMinor(h)
		if v <= 0 {
			continue // a zero or negative position costs nothing to hold
		}
		out.TotalMinor += v
		if h.ExpenseRatioBps <= 0 {
			continue
		}
		out.CoveredMinor += v
		weighted += float64(v) * float64(h.ExpenseRatioBps)
		out.AnnualMinor += int64(math.Round(float64(v) * float64(h.ExpenseRatioBps) / 10_000))
	}
	if out.CoveredMinor > 0 {
		out.WeightedBps = weighted / float64(out.CoveredMinor)
	}
	return out
}

// HoldingFeeMinor is one holding's annual expense-ratio cost at current value.
func HoldingFeeMinor(h Holding) int64 {
	if h.ExpenseRatioBps <= 0 {
		return 0
	}
	v := HoldingValueMinor(h)
	if v <= 0 {
		return 0
	}
	return int64(math.Round(float64(v) * float64(h.ExpenseRatioBps) / 10_000))
}
