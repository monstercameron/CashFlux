// SPDX-License-Identifier: MIT

package budgeting

// incomesplit.go answers one question: where did the month's money go?
//
// The budgets hero used to answer a different one — how does total spend compare
// with the sum of my limits — and that question has no stable meaning. Its
// denominator is however many budgets you happen to have created, so it moves
// when you add a budget without spending a penny, and it says "over budget" the
// moment you spend in a category you never budgeted, even if you are comfortably
// inside your income.
//
// It was also arithmetically not the overage. Spend minus limits NETS one
// budget's overspend against another's headroom: a household $3,108.70 over on
// bars and $722.47 under on bills was told it was $2,486.23 over, a number that
// appears nowhere in the rows above it and describes nothing anyone can act on.
// Money in one budget is not available to another unless somebody moves it, which
// is what Cover is for.
//
// Income is the denominator that cannot be gamed. You cannot change it by
// creating a budget, and "did I spend more than I made" is the question people
// actually open this page with.

// IncomeSplit is the month's money, divided by where it went. The parts are
// disjoint and — with Left — sum to Income exactly, which is what lets the bar be
// drawn as one honest track rather than several bars that do not agree.
type IncomeSplit struct {
	// Income is the denominator: money in for the period, plus anything carried.
	Income int64
	// WithinLimits is spending that stayed inside a budget's limit.
	WithinLimits int64
	// OverLimits is spending past a limit, summed per budget. This is the real
	// overage — the same number the rows show — never netted against headroom
	// elsewhere.
	OverLimits int64
	// Untracked is spending in categories no budget watches. It was a footnote
	// under the list before; it is the largest slice in plenty of real months, and
	// leaving it out of the headline is how a page can say "97% budgeted" while
	// thousands leave unaccounted.
	Untracked int64
	// Savings is income assigned to savings and investment targets. Not spending:
	// it is money that stayed, and colouring it as an outflow makes prudence look
	// like a problem.
	Savings int64
	// Left is whatever income remains. Negative means the month spent more than it
	// made, which is the one fact the old hero could not state.
	Left int64
}

// Overspent reports whether more money left than came in.
func (s IncomeSplit) Overspent() bool { return s.Left < 0 }

// Spent is everything that actually left, whatever it was budgeted against.
func (s IncomeSplit) Spent() int64 { return s.WithinLimits + s.OverLimits + s.Untracked }

// SplitIncome divides income into where it went.
//
// withinLimits and overLimits come from the per-budget rollup, so the parts agree
// with the rows by construction rather than by a second calculation that has to
// be kept in step.
func SplitIncome(income, withinLimits, overLimits, untracked, savings int64) IncomeSplit {
	s := IncomeSplit{
		Income: income, WithinLimits: withinLimits, OverLimits: overLimits,
		Untracked: untracked, Savings: savings,
	}
	s.Left = income - s.Spent() - savings
	return s
}

// SplitFromRollup builds the split from a rollup, deriving within-limits rather
// than asking the caller for it.
//
// The rollup's SpentMinor already INCLUDES the overspend, so within-limits is
// spend minus over. Passing both separately would let a caller hand over numbers
// that do not add up, and a bar whose segments do not sum to its own total is
// worse than no bar.
func SplitFromRollup(r RollupSummary, income, untracked, savings int64) IncomeSplit {
	within := r.SpentMinor - r.OverMinor
	if within < 0 {
		within = 0
	}
	return SplitIncome(income, within, r.OverMinor, untracked, savings)
}

// Pct returns part as a whole percentage of Income, clamped to 0..100 and never
// dividing by a zero or negative income.
//
// A month with no recorded income is not a month where everything is 100% of
// nothing: the caller draws the empty state instead, which is why this returns
// zero rather than something that looks like a measurement.
func (s IncomeSplit) Pct(part int64) int {
	if s.Income <= 0 || part <= 0 {
		return 0
	}
	p := int(part * 100 / s.Income)
	if p > 100 {
		return 100
	}
	return p
}
