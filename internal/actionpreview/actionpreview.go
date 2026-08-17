// SPDX-License-Identifier: MIT

// Package actionpreview describes what an action would do, before it does it
// (WF6).
//
// Recommendations in this app arrive as verbs — pay this down, move this here,
// apply this rule — and the reader has to take on faith that the suggester
// considered everything else. This is the shape that removes the faith: a list
// of the figures the action moves, with their before and after, and an explicit
// statement of the ones it leaves alone.
//
// # Why unchanged metrics are stated rather than omitted
//
// A preview listing only what moved leaves the reader wondering about
// everything it did not mention. "Goal funding unchanged" is not filler — it is
// the answer to the question the reader was already asking, and its absence is
// indistinguishable from nobody having checked.
//
// # Why direction is declared, not inferred
//
// Whether a number going up is good depends entirely on which number it is. A
// rising balance is good, a rising utilization is not, and no amount of looking
// at the value tells you which you have. Every metric declares it, and one that
// does not is reported as CHANGED with no verdict rather than being guessed at.
package actionpreview

import "sort"

// Direction says whether a change is an improvement.
type Direction string

const (
	// DirectionBetter and DirectionWorse are judgements, only available when the
	// metric declared which way is up.
	DirectionBetter Direction = "better"
	DirectionWorse  Direction = "worse"
	// DirectionUnknown means the figure moved and the app will not say whether
	// that is good. Honest, and distinct from unchanged.
	DirectionUnknown Direction = "unknown"
	// DirectionSame means the action does not touch it.
	DirectionSame Direction = "same"
)

// Goodness is how a metric's direction should be read.
type Goodness string

const (
	// HigherBetter is a metric where up is an improvement (a balance, a runway).
	HigherBetter Goodness = "higher"
	// LowerBetter is one where down is (debt, utilization, interest paid).
	LowerBetter Goodness = "lower"
	// Neutral is a metric with no inherent direction — a count, a date that is
	// merely different. It reports as changed with no verdict.
	Neutral Goodness = "neutral"
)

// Metric is one figure an action touches, or does not.
type Metric struct {
	// Name reads as a noun phrase in the reader's language ("card utilization",
	// "months of runway"), because it is used verbatim in the preview.
	Name string
	// Before and After are the figure's values, in whatever unit the caller is
	// working in — minor units, months, percent. This package compares and never
	// formats: a package that formatted would need to know about currencies and
	// locales to describe a change to a month count.
	Before, After int64
	// Goodness says which way is up.
	Goodness Goodness
	// Display, when set, is a pair of already-formatted strings the surface should
	// show instead of the raw numbers.
	DisplayBefore, DisplayAfter string
}

// Change is one metric's movement.
type Change struct {
	Metric    Metric
	Direction Direction
	// DeltaAbs is the size of the move, always positive — the sign lives in
	// Direction, so a surface never has to work out whether a negative delta on a
	// lower-is-better metric is good news.
	DeltaAbs int64
}

// Preview is everything an action would do.
type Preview struct {
	// Changed are the metrics that move, ordered worse-first.
	//
	// Worse first, deliberately. A preview that leads with its improvements is an
	// advertisement; one that leads with its costs is a decision aid, and the
	// reader can stop reading as soon as they have seen the thing that would put
	// them off.
	Changed []Change
	// Unchanged names the metrics the action leaves alone, sorted.
	Unchanged []string
}

// Any reports whether the action changes anything at all.
func (p Preview) Any() bool { return len(p.Changed) > 0 }

// Worsens reports whether anything gets worse — the fact a reader most needs
// before pressing a button.
func (p Preview) Worsens() bool {
	for _, c := range p.Changed {
		if c.Direction == DirectionWorse {
			return true
		}
	}
	return false
}

// Build turns a set of before/after metrics into a preview.
func Build(metrics []Metric) Preview {
	var p Preview
	for _, m := range metrics {
		if m.Name == "" {
			continue
		}
		if m.Before == m.After {
			p.Unchanged = append(p.Unchanged, m.Name)
			continue
		}
		delta := m.After - m.Before
		if delta < 0 {
			delta = -delta
		}
		p.Changed = append(p.Changed, Change{
			Metric: m, Direction: directionOf(m), DeltaAbs: delta,
		})
	}
	sort.Strings(p.Unchanged)
	// Worse first, then unknown, then better; ties by name so the same action
	// always previews identically.
	rank := map[Direction]int{DirectionWorse: 0, DirectionUnknown: 1, DirectionBetter: 2, DirectionSame: 3}
	sort.SliceStable(p.Changed, func(i, j int) bool {
		if rank[p.Changed[i].Direction] != rank[p.Changed[j].Direction] {
			return rank[p.Changed[i].Direction] < rank[p.Changed[j].Direction]
		}
		return p.Changed[i].Metric.Name < p.Changed[j].Metric.Name
	})
	return p
}

// directionOf judges a move, or declines to.
func directionOf(m Metric) Direction {
	up := m.After > m.Before
	switch m.Goodness {
	case HigherBetter:
		if up {
			return DirectionBetter
		}
		return DirectionWorse
	case LowerBetter:
		if up {
			return DirectionWorse
		}
		return DirectionBetter
	default:
		// The figure moved; whether that is good is not this package's to say.
		return DirectionUnknown
	}
}
