// SPDX-License-Identifier: MIT

// Package marginal answers "what does another dollar here actually earn" for
// each place spare cash could go (WF10).
//
// The allocation engine already ranks destinations and explains its scoring. The
// thing it could not do is put them on one scale a person can check: paying down
// a 22% card and topping up a 4% savings account are not "rank 1 and rank 4",
// they are $220 a year and $40 a year, and the second phrasing is the one
// somebody can disagree with.
//
// # The rule this package is built around
//
// Only destinations with a RATE get a figure. Paying down debt avoids interest;
// savings earn it; both are arithmetic. An underfunded budget, a goal with a
// date, an emergency reserve — these have real value that is not interest, and
// attaching a dollar figure to them would be inventing a number to make a table
// look complete.
//
// A table with blanks in it is honest. A table where every row has a number and
// half of them are made up is not, and it is worse precisely because it looks
// finished.
package marginal

import (
	"math"
	"sort"
)

// Kind is what sort of destination a dollar is going to.
type Kind string

const (
	// KindDebt is paying down a balance that charges interest.
	KindDebt Kind = "debt"
	// KindSavings is money that earns a stated return.
	KindSavings Kind = "savings"
	// KindOther is everything whose value is real but not a rate — an emergency
	// reserve, a goal by a date, an underfunded budget.
	KindOther Kind = "other"
)

// Destination is one place spare cash could go.
type Destination struct {
	ID   string
	Name string
	Kind Kind
	// AnnualRatePct is the APR being avoided (debt) or earned (savings). Zero
	// means no rate is known, which is reported as unknown rather than as zero
	// benefit — "we do not know what this card charges" and "this card charges
	// nothing" are different facts and only one of them is ever true.
	AnnualRatePct float64
	// CapacityMinor is how much this destination can absorb: the balance owed, the
	// room left in a goal, the shortfall in a budget. Zero means no limit.
	CapacityMinor int64
}

// Benefit is what a dollar amount does at one destination over a year.
type Benefit struct {
	Destination Destination
	// AmountMinor is what was allocated here.
	AmountMinor int64
	// AnnualMinor is the interest avoided or earned in a year on that amount, and
	// Known says whether it could be computed at all.
	AnnualMinor int64
	Known       bool
	// Capped is true when the destination could not absorb the whole amount, so a
	// surface can say why the figure is smaller than the allocation.
	Capped bool
}

// EffectiveMinor is the part of the allocation this destination actually took.
func (b Benefit) EffectiveMinor() int64 {
	if b.Capped && b.Destination.CapacityMinor > 0 && b.Destination.CapacityMinor < b.AmountMinor {
		return b.Destination.CapacityMinor
	}
	return b.AmountMinor
}

// Compute works out a year's benefit of putting amountMinor at d.
//
// Reports Known=false — never a zero — when there is no rate to work from. A
// zero would sort alongside a genuinely useless destination and read as a
// measurement, when it is an absence.
func Compute(d Destination, amountMinor int64) Benefit {
	b := Benefit{Destination: d, AmountMinor: amountMinor}
	if amountMinor <= 0 {
		return b
	}
	if d.CapacityMinor > 0 && d.CapacityMinor < amountMinor {
		b.Capped = true
	}
	if d.Kind == KindOther || d.AnnualRatePct <= 0 {
		return b
	}
	eff := b.EffectiveMinor()
	b.AnnualMinor = int64(math.Round(float64(eff) * d.AnnualRatePct / 100))
	b.Known = true
	return b
}

// Compare works out the benefit of the same amount at every destination and
// orders them best first.
//
// Destinations with no computable benefit sort LAST but are still returned, and
// that is deliberate: dropping them would quietly remove the emergency fund from
// a list of places to put money, which is a recommendation dressed as a filter.
func Compare(dests []Destination, amountMinor int64) []Benefit {
	out := make([]Benefit, 0, len(dests))
	for _, d := range dests {
		out = append(out, Compute(d, amountMinor))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Known != out[j].Known {
			return out[i].Known
		}
		if out[i].AnnualMinor != out[j].AnnualMinor {
			return out[i].AnnualMinor > out[j].AnnualMinor
		}
		return out[i].Destination.ID < out[j].Destination.ID
	})
	return out
}

// Lock holds one destination at a fixed amount and reports what is left for
// everything else (WF10's "lock a destination and rerun").
//
// Returns ok=false when the lock exceeds the pot: a plan that allocates money
// nobody has is not a plan, and silently trimming the lock would answer a
// question the user did not ask.
func Lock(totalMinor, lockedMinor int64) (remainingMinor int64, ok bool) {
	if lockedMinor < 0 || totalMinor < 0 || lockedMinor > totalMinor {
		return 0, false
	}
	return totalMinor - lockedMinor, true
}

// BestKnown returns the highest-benefit destination that has a figure at all.
//
// Reports ok=false when nothing has a rate, rather than returning the first
// row — the top of a list sorted by an absent value is not a recommendation.
func BestKnown(bs []Benefit) (Benefit, bool) {
	for _, b := range bs {
		if b.Known {
			return b, true
		}
	}
	return Benefit{}, false
}

// SpreadMinor is how much better the best known destination is than the second.
//
// The figure that says whether the choice matters. A $3-a-year gap between two
// destinations is a decision not worth making, and a ranking that presents it
// with the same confidence as a $300 gap is wasting somebody's attention.
func SpreadMinor(bs []Benefit) (int64, bool) {
	var first, second Benefit
	found := 0
	for _, b := range bs {
		if !b.Known {
			continue
		}
		switch found {
		case 0:
			first = b
		case 1:
			second = b
		}
		found++
		if found == 2 {
			break
		}
	}
	if found < 2 {
		return 0, false
	}
	return first.AnnualMinor - second.AnnualMinor, true
}
