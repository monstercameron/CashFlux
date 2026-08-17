// SPDX-License-Identifier: MIT

package portfolio

import (
	"math"
	"sort"
	"time"
)

// ─── FP-T1c: what the portfolio actually RETURNED ────────────────────────────
//
// The investments growth chart plots a BALANCE. A balance that doubled tells you
// nothing about performance, because it doubles the same way whether the market
// went up or you paid more in — and those are opposite facts about how the
// portfolio is doing.
//
// Two returns answer two different questions, and shipping only one is how
// people end up believing the wrong thing about their own money:
//
//   - TIME-weighted (TWR) removes the effect of WHEN money went in. It is the
//     number that answers "how did these investments do", and the one a fund
//     quotes, precisely because a fund does not control its investors' deposits.
//   - MONEY-weighted (IRR) includes that effect. It answers "how did I do",
//     which is a different and often much worse number — buying heavily just
//     before a fall is a real outcome the investor lived through and TWR hides
//     it completely.
//
// Both are annualized so a six-month figure and a six-year figure can be read
// against each other, which is the only way a return is comparable to anything.

// Flow is a dated external cash movement into or out of the portfolio. Positive
// is money IN (a contribution), negative is money OUT (a withdrawal).
//
// External is the load-bearing word: a dividend reinvested inside the account is
// not a flow, it is a return. Counting it as a contribution would make the
// portfolio look like it grew because you fed it, which is the exact inversion
// these functions exist to prevent.
type Flow struct {
	Date        time.Time
	AmountMinor int64
}

// Valuation is the portfolio's total market value on a date.
type Valuation struct {
	Date       time.Time
	ValueMinor int64
}

// MinReturnDays is the shortest span over which a return is reported.
//
// Thirty days, because annualizing a shorter window multiplies its noise by the
// same factor it multiplies the return: three good days annualize to a
// spectacular and completely meaningless number, and a figure that reads as
// spectacular is one people act on.
const MinReturnDays = 30

// Performance is a portfolio's return over a period, both ways.
type Performance struct {
	// TimeWeightedPct is the annualized time-weighted return — "how did the
	// investments do", independent of when money was added.
	TimeWeightedPct float64
	// MoneyWeightedPct is the annualized internal rate of return — "how did I
	// do", including the timing of contributions.
	MoneyWeightedPct float64
	// TWRKnown / IRRKnown say whether each figure means anything. They are
	// separate because the two can genuinely differ: a period with valuations but
	// no flows has a TWR and a degenerate IRR, and reporting a zero for the
	// missing one would read as "you made nothing".
	TWRKnown, IRRKnown bool
	// Days is the span covered, and Flows/Valuations how much evidence there was
	// — so a surface can say "over 94 days, 3 contributions" rather than
	// presenting a number with no provenance.
	Days       int
	Flows      int
	Valuations int
}

// GapPct is how far the investor's experience diverged from the investments'
// own performance. Negative means the timing of contributions cost money —
// which is the common case and the one worth surfacing.
//
// Reports ok=false unless BOTH figures are known; a gap computed against a
// missing number is not a gap.
func (p Performance) GapPct() (float64, bool) {
	if !p.TWRKnown || !p.IRRKnown {
		return 0, false
	}
	return p.MoneyWeightedPct - p.TimeWeightedPct, true
}

// Returns computes both returns from dated valuations and external flows.
//
// valuations must include the period's start and end; anything in between adds
// TWR precision by splitting the period at that point. Flows falling outside the
// valuation span are ignored rather than folded into an endpoint — a
// contribution made before the period began is not part of this period's return,
// and pretending otherwise is how a strong quarter gets attributed to a deposit
// made a year earlier.
func Returns(vals []Valuation, flows []Flow) Performance {
	var p Performance
	if len(vals) < 2 {
		return p
	}
	v := append([]Valuation(nil), vals...)
	sort.SliceStable(v, func(i, j int) bool { return v[i].Date.Before(v[j].Date) })
	start, end := v[0], v[len(v)-1]
	days := int(end.Date.Sub(start.Date).Hours() / 24)
	if days < MinReturnDays {
		return p
	}
	p.Days, p.Valuations = days, len(v)

	inWindow := make([]Flow, 0, len(flows))
	for _, f := range flows {
		if f.Date.Before(start.Date) || f.Date.After(end.Date) || f.AmountMinor == 0 {
			continue
		}
		inWindow = append(inWindow, f)
	}
	sort.SliceStable(inWindow, func(i, j int) bool { return inWindow[i].Date.Before(inWindow[j].Date) })
	p.Flows = len(inWindow)

	if twr, ok := timeWeighted(v, inWindow, days); ok {
		p.TimeWeightedPct, p.TWRKnown = twr, true
	}
	if irr, ok := moneyWeighted(v, inWindow); ok {
		p.MoneyWeightedPct, p.IRRKnown = irr, true
	}
	return p
}

// timeWeighted chains the return of each sub-period between valuations, so a
// contribution changes the balance without registering as performance.
//
// A sub-period whose starting value is zero is SKIPPED rather than treated as an
// infinite return: an account funded from nothing has no percentage return over
// the interval in which it was funded, and the alternative is a division by zero
// that propagates through the whole chain.
func timeWeighted(v []Valuation, flows []Flow, days int) (float64, bool) {
	growth := 1.0
	measured := false
	for i := 1; i < len(v); i++ {
		prev, cur := v[i-1], v[i]
		var netFlow int64
		for _, f := range flows {
			if f.Date.After(prev.Date) && !f.Date.After(cur.Date) {
				netFlow += f.AmountMinor
			}
		}
		if prev.ValueMinor <= 0 {
			continue
		}
		// The sub-period's return excludes the money that arrived during it.
		r := (float64(cur.ValueMinor) - float64(netFlow)) / float64(prev.ValueMinor)
		if r <= 0 {
			// A total loss inside a sub-period makes the chained product zero and
			// every later sub-period meaningless. Reporting "we cannot say" beats
			// reporting -100% for a portfolio that later recovered.
			return 0, false
		}
		growth *= r
		measured = true
	}
	if !measured {
		return 0, false
	}
	return annualize(growth, days)
}

// moneyWeighted solves the annualized rate that makes the discounted flows and
// the ending value equal the starting value — the IRR.
//
// Bisection, not Newton. Newton is faster and can diverge on the irregular cash
// flows a real portfolio produces; a return figure that occasionally comes back
// as NaN is worse than one that always takes forty iterations.
func moneyWeighted(v []Valuation, flows []Flow) (float64, bool) {
	start, end := v[0], v[len(v)-1]
	if start.ValueMinor <= 0 && len(flows) == 0 {
		return 0, false
	}
	// npv is the present value at the start date of everything that happened.
	npv := func(rate float64) float64 {
		yrs := func(t time.Time) float64 { return t.Sub(start.Date).Hours() / 24 / 365.25 }
		out := float64(start.ValueMinor)
		for _, f := range flows {
			out += float64(f.AmountMinor) / math.Pow(1+rate, yrs(f.Date))
		}
		return out - float64(end.ValueMinor)/math.Pow(1+rate, yrs(end.Date))
	}
	lo, hi := -0.9999, 10.0
	fLo, fHi := npv(lo), npv(hi)
	if math.IsNaN(fLo) || math.IsNaN(fHi) || fLo*fHi > 0 {
		// No sign change in the bracket means no root in it. Returning a bracket
		// endpoint would be a fabricated answer; a portfolio that lost more than
		// 99.99% or gained more than 1000% a year is outside what this reports.
		return 0, false
	}
	for range 200 {
		mid := (lo + hi) / 2
		f := npv(mid)
		if math.Abs(f) < 1e-6 || hi-lo < 1e-9 {
			return mid * 100, true
		}
		if f*fLo < 0 {
			hi = mid
		} else {
			lo, fLo = mid, f
		}
	}
	return (lo + hi) / 2 * 100, true
}

// annualize turns a total growth multiple over `days` into an annual rate.
//
// Both directions matter: a six-month result is scaled UP and a six-year result
// scaled DOWN, because the point of annualizing is comparability, not
// flattering a short window.
func annualize(growth float64, days int) (float64, bool) {
	if growth <= 0 || days <= 0 {
		return 0, false
	}
	years := float64(days) / 365.25
	if years <= 0 {
		return 0, false
	}
	return (math.Pow(growth, 1/years) - 1) * 100, true
}
