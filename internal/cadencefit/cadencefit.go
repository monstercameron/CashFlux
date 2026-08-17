// SPDX-License-Identifier: MIT

// Package cadencefit spots a budget whose PERIOD disagrees with the rhythm of
// the spending it tracks (EC-10).
//
// A monthly budget for something billed once a year is not wrong about the
// money. It is wrong about the rhythm, and the consequence is that it reads as
// blown once and unused eleven times — eleven months of a green bar that means
// nothing, then one month of a red one that means "this was always going to
// happen". Neither reading tells the household anything, and both train them to
// ignore the bar.
//
// # It is about concentration, not variability
//
// Groceries vary month to month and belong on a monthly budget; a car insurance
// premium arrives twice a year and does not. The difference is not how much the
// amounts move, it is how many periods have any spending in them at all. This
// package measures that and nothing else.
//
// # It does not duplicate the sinking-fund nudge
//
// SMART-BL9 already acts on DECLARED recurring items, where the cadence is
// known because somebody entered it. This is for the spending nobody declared —
// the rhythm has to be discovered from what actually posted — and it points at
// the same remedy rather than inventing a second one.
package cadencefit

// MinPeriods is how much history is needed before a rhythm can be claimed.
//
// Twelve periods. Below a full year, "annual" is unprovable: one payment in the
// six months on record is one payment, not a yearly bill, and calling it one
// would restructure a budget on a coincidence.
const MinPeriods = 12

// MaxActiveShare is the share of periods that may contain spending before the
// rhythm counts as ordinary.
//
// A third. Spending in four months of twelve is lumpy enough to plan for
// differently; spending in five is just a category the household does not use
// every month, and rebuilding a budget around that would be noise.
const MaxActiveShare = 1.0 / 3.0

// MinTotalMinor is the smallest yearly total worth restructuring a budget over.
//
// Two hundred currency units. Below that the advice costs more attention than
// the mistake does — the whole finding is "your budget is shaped wrong", and
// nobody should reshape a budget over $40 a year.
const MinTotalMinor = 200_00

// Shape is the rhythm the spending actually has.
type Shape string

const (
	// ShapeOrdinary: spending turns up in most periods. A monthly budget fits.
	ShapeOrdinary Shape = "ordinary"
	// ShapeQuarterly: roughly every third period.
	ShapeQuarterly Shape = "quarterly"
	// ShapeAnnual: once or twice a year.
	ShapeAnnual Shape = "annual"
	// ShapeIrregular: concentrated, but on no rhythm worth naming.
	ShapeIrregular Shape = "irregular"
)

// Fit is what the spending's rhythm says about the budget tracking it.
type Fit struct {
	// Known is false when there is too little history to judge. Reported rather
	// than ShapeOrdinary, because "it looks fine" and "we have not seen enough"
	// are different answers and only the first is reassuring.
	Known bool
	Shape Shape
	// ActivePeriods and Periods are the evidence: how many periods had any
	// spending, out of how many were observed.
	ActivePeriods, Periods int
	// TotalMinor is what went through over the whole span.
	TotalMinor int64
	// SuggestedMonthlyMinor is what setting the money aside each period would
	// cost — the sinking-fund figure, which is the remedy this points at.
	SuggestedMonthlyMinor int64
}

// Mismatched reports whether the budget's rhythm is worth telling somebody
// about: enough history, concentrated spending, and enough money to matter.
func (f Fit) Mismatched() bool {
	return f.Known && f.Shape != ShapeOrdinary && f.TotalMinor >= MinTotalMinor
}

// Assess reads a per-period spend series, oldest first, in magnitudes.
//
// The series must have one entry per period INCLUDING the empty ones — the empty
// periods are the entire signal, and a caller that passes only the periods with
// spending would describe every category as annual.
func Assess(periods []int64) Fit {
	var f Fit
	if len(periods) < MinPeriods {
		return f
	}
	f.Known = true
	f.Periods = len(periods)
	var active []int
	for i, v := range periods {
		if v <= 0 {
			continue
		}
		f.ActivePeriods++
		f.TotalMinor += v
		active = append(active, i)
	}
	if f.ActivePeriods == 0 {
		// A budget with nothing in it is a different problem, and not this one.
		f.Shape = ShapeOrdinary
		return f
	}
	f.SuggestedMonthlyMinor = f.TotalMinor / int64(f.Periods)

	if float64(f.ActivePeriods) > float64(f.Periods)*MaxActiveShare {
		f.Shape = ShapeOrdinary
		return f
	}
	f.Shape = classify(active)
	return f
}

// classify names the rhythm of the periods that had spending.
//
// Evenness is what separates a cadence from a burst: four payments three months
// apart are quarterly, and four payments in one fortnight are one event that
// happened to span a period boundary. Calling the second "quarterly" would have
// the household set money aside for a repeat that is not coming.
func classify(active []int) Shape {
	if len(active) <= 2 {
		return ShapeAnnual
	}
	gaps := make([]int, 0, len(active)-1)
	for i := 1; i < len(active); i++ {
		gaps = append(gaps, active[i]-active[i-1])
	}
	minGap, maxGap := gaps[0], gaps[0]
	sum := 0
	for _, g := range gaps {
		if g < minGap {
			minGap = g
		}
		if g > maxGap {
			maxGap = g
		}
		sum += g
	}
	// Even means every gap is within one period of every other. A stricter test
	// would reject a bill that slipped a month; a looser one would call anything
	// quarterly.
	if maxGap-minGap > 1 {
		return ShapeIrregular
	}
	avg := float64(sum) / float64(len(gaps))
	switch {
	case avg >= 2.5 && avg <= 4.5:
		return ShapeQuarterly
	case avg > 4.5:
		return ShapeAnnual
	}
	return ShapeIrregular
}
