// SPDX-License-Identifier: MIT

// Package actionrank orders the things a household could do next, and says why
// one comes above another (WF-SM3).
//
// It is not internal/allocate. That answers "where should this dollar go" and
// ranks money destinations; this answers "which of these jobs should I do
// first", and the jobs are not all about moving money — cancelling a
// subscription, calling a lender, fixing a category.
//
// # The ranking is the easy half
//
// Any weighted sum produces an order. The ticket's actual requirement is the
// sentence after it: WHY does this outrank that. A list of scores does not
// answer it — a reader comparing two rows of numbers is doing the work the app
// was supposed to do — so Why names the single criterion that decided the pair,
// and refuses when nothing really did.
//
// # Confidence multiplies
//
// A saving you are unsure of is worth less than the same saving you are certain
// of, which sounds like an argument for adding confidence to the score. It is
// not: added, a completely pointless action you are perfectly sure about climbs
// the list on certainty alone. Confidence scales the benefit it applies to and
// contributes nothing by itself.
package actionrank

import (
	"math"
	"sort"
)

// Effort is how much work an action takes.
type Effort int

const (
	// EffortLow: a tap, a toggle, one form.
	EffortLow Effort = iota
	// EffortMedium: a phone call, a document, a decision with somebody else.
	EffortMedium
	// EffortHigh: a move, a refinance, a change of job or provider.
	EffortHigh
)

// Criterion names one input to the ranking, so a reason can point at it.
type Criterion string

const (
	CritMonthly    Criterion = "monthly"
	CritOneTime    Criterion = "onetime"
	CritEffort     Criterion = "effort"
	CritReversible Criterion = "reversible"
	CritUrgency    Criterion = "urgency"
	CritConfidence Criterion = "confidence"
)

// MonthsHorizon is how far ahead a recurring saving is counted when it is
// weighed against a one-off.
//
// Twelve months. A month's saving repeats and a one-off does not, so comparing
// the two at face value would rank a single $200 rebate above $50 a month
// forever — which is wrong within four months and wildly wrong within a year.
// Twelve is the span a household can actually picture; counting to infinity
// would make every recurring action beat every one-off regardless of size.
const MonthsHorizon = 12

// UrgentWithinDays is when a deadline starts to matter to the order.
//
// Thirty days. Beyond that "urgent" is a mood rather than a constraint, and
// sorting by it would just be sorting by how the item was written.
const UrgentWithinDays = 30

// CloseCallPct is the margin below which two actions are not meaningfully
// ordered.
//
// Five percent of the leader's score. Below it the order is an artifact of the
// weights, and inventing a reason for it would be dressing up a coin-flip as
// analysis.
const CloseCallPct = 5.0

// Action is one thing the household could do.
type Action struct {
	ID   string
	Name string
	// MonthlyImpactMinor is money saved or gained EVERY month.
	MonthlyImpactMinor int64
	// OneTimeImpactMinor is money saved or gained once.
	OneTimeImpactMinor int64
	Effort             Effort
	// Reversible is whether it can be undone if it turns out wrong.
	Reversible bool
	// UrgencyDays is how long until the chance is gone. Negative means no
	// deadline — which is not the same as a deadline far away, and is why this is
	// not just a large number.
	UrgencyDays int
	// Confidence is how sure the impact figures are, 0..1. Zero means the numbers
	// are a guess; the action can still be listed, but it will not lead on them.
	Confidence float64
	// HasDeadline distinguishes "no deadline" from "a deadline of zero days".
	HasDeadline bool
}

// Ranked is one action with its score and the parts it came from.
type Ranked struct {
	Action    Action
	Score     float64
	Breakdown map[Criterion]float64
}

// Rank orders actions best-first. Ties break on ID so the same set always
// produces the same list — a ranking that reshuffles between renders cannot be
// re-read, and a reader who cannot re-read it will not trust it.
func Rank(actions []Action) []Ranked {
	out := make([]Ranked, 0, len(actions))
	for _, a := range actions {
		s, b := score(a)
		out = append(out, Ranked{Action: a, Score: s, Breakdown: b})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Action.ID < out[j].Action.ID
	})
	return out
}

// valueMinor is what an action is worth over the horizon, in money, scaled by
// how sure we are of it.
//
// Confidence MULTIPLIES the benefit rather than adding to the score: added, a
// pointless action somebody is perfectly sure about would climb the list on
// certainty alone.
func valueMinor(a Action) float64 {
	raw := float64(a.MonthlyImpactMinor)*MonthsHorizon + float64(a.OneTimeImpactMinor)
	return raw * clamp01(a.Confidence)
}

// score builds the total and its parts. Effort and reversibility are COSTS: they
// can only pull a score down, never lift one. An action that is easy and
// reversible is not thereby worth doing — it is just cheap to try.
func score(a Action) (float64, map[Criterion]float64) {
	b := map[Criterion]float64{}
	// Money, on a log scale: the difference between $5 and $50 a month matters
	// far more than between $500 and $545, and a linear scale would let one large
	// item flatten every other criterion to noise.
	v := valueMinor(a)
	money := 0.0
	if v > 0 {
		money = math.Log10(1 + v/100) // minor units → currency units
	}
	b[CritMonthly] = math.Log10(1+float64(a.MonthlyImpactMinor)*MonthsHorizon/100) * clamp01(a.Confidence)
	b[CritOneTime] = math.Log10(1+float64(a.OneTimeImpactMinor)/100) * clamp01(a.Confidence)
	b[CritConfidence] = clamp01(a.Confidence)

	urgency := 0.0
	if a.HasDeadline && a.UrgencyDays >= 0 && a.UrgencyDays <= UrgentWithinDays {
		// Nearer is stronger, and today is strongest.
		urgency = float64(UrgentWithinDays-a.UrgencyDays) / float64(UrgentWithinDays)
	}
	b[CritUrgency] = urgency

	effortCost := map[Effort]float64{EffortLow: 0, EffortMedium: 0.35, EffortHigh: 0.8}[a.Effort]
	b[CritEffort] = -effortCost
	irreversibleCost := 0.0
	if !a.Reversible {
		irreversibleCost = 0.3
	}
	b[CritReversible] = -irreversibleCost

	total := money + urgency - effortCost - irreversibleCost
	return total, b
}

// Reason says why one action outranks another.
type Reason struct {
	// Criterion is the single thing that decided it. Empty when TooClose.
	Criterion Criterion
	// MarginPct is how far ahead the winner is, as a percentage of its own score.
	MarginPct float64
	// TooClose is true when the gap is inside CloseCallPct — the honest answer
	// being that these two are interchangeable, not that one is better for a
	// reason nobody could act on.
	TooClose bool
}

// Why explains why a outranks b. It names ONE criterion — the one whose
// difference contributed most to the gap — because a reason listing every input
// is the score sheet again, and the reader is back to doing the comparison
// themselves.
//
// Callers pass the pair in ranked order; passing them the other way round
// reports the gap as a close call rather than inventing a reason for a losing
// action.
func Why(a, b Ranked) Reason {
	gap := a.Score - b.Score
	den := math.Abs(a.Score)
	if den < 0.0001 {
		den = 1
	}
	margin := gap / den * 100
	if gap <= 0 || margin < CloseCallPct {
		return Reason{MarginPct: math.Max(margin, 0), TooClose: true}
	}
	best, bestDiff := Criterion(""), 0.0
	// Fixed order so an exact tie between two criteria always resolves the same
	// way; a reason that changes between renders is worse than no reason.
	for _, c := range []Criterion{CritMonthly, CritOneTime, CritUrgency, CritEffort, CritReversible, CritConfidence} {
		if d := a.Breakdown[c] - b.Breakdown[c]; d > bestDiff {
			best, bestDiff = c, d
		}
	}
	if best == "" {
		// The gap came entirely from parts that do not favour the winner — which
		// means nothing about it can be named honestly.
		return Reason{MarginPct: margin, TooClose: true}
	}
	return Reason{Criterion: best, MarginPct: margin}
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
