// SPDX-License-Identifier: MIT

// Package goalcompare answers two questions the goals Compare surface could not
// (C398): which goals can be compared and why, and what happens to both landing
// dates when you change which one gets funded first.
//
// The first was a copy problem hiding a code problem. The picker offered a
// subset of goals and said nothing about the rule, because there was no rule —
// just a condition inlined in a render function. A filter with no name cannot be
// explained to the reader, so it is named here, with a reason per exclusion.
//
// The second is the question the surface exists to answer. Two columns of
// figures show what each goal costs; they do not show the trade. Funding order
// is the actual decision — with one surplus and two goals, choosing which goes
// first moves both dates, and only one of those movements is the one you wanted.
package goalcompare

import (
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Reason names why a goal cannot take part in a comparison.
type Reason string

const (
	// ReasonNone means the goal is eligible.
	ReasonNone Reason = ""
	// ReasonArchived: the goal is archived, so its dates are history.
	ReasonArchived Reason = "archived"
	// ReasonNotFinancial: the goal has no money dimension (a habit or a
	// milestone), so there is no landing date to compare.
	ReasonNotFinancial Reason = "not-financial"
	// ReasonNoTarget: the goal has no target amount, so "when does it land" has
	// no answer — there is nothing to land on.
	ReasonNoTarget Reason = "no-target"
)

// Eligibility reports whether a goal can appear in a comparison, and why not
// when it cannot. The reasons are ordered from most to least fundamental so a
// goal failing several is described by the one the reader should act on first:
// un-archiving a habit goal will not make it comparable, but giving a financial
// goal a target will.
func Eligibility(g domain.Goal) Reason {
	switch {
	case !g.EffectiveKind().IsFinancial():
		return ReasonNotFinancial
	case g.Archived:
		return ReasonArchived
	case g.TargetAmount.Amount <= 0:
		return ReasonNoTarget
	}
	return ReasonNone
}

// Eligible reports whether a goal can be compared.
func Eligible(g domain.Goal) bool { return Eligibility(g) == ReasonNone }

// Excluded pairs a goal with why it was left out, for a UI that explains itself
// instead of silently shortening a list.
type Excluded struct {
	Goal   domain.Goal
	Reason Reason
}

// Partition splits goals into those that can be compared and those that cannot,
// each with its reason, preserving input order.
func Partition(goals []domain.Goal) ([]domain.Goal, []Excluded) {
	var ok []domain.Goal
	var out []Excluded
	for _, g := range goals {
		if r := Eligibility(g); r == ReasonNone {
			ok = append(ok, g)
		} else {
			out = append(out, Excluded{Goal: g, Reason: r})
		}
	}
	return ok, out
}

// ─── funding order ───────────────────────────────────────────────────────────

// MaxRaceMonths bounds the simulation. Fifty years is well past the horizon of
// any plan a person would act on, and a bound is what keeps a goal funded at a
// rounding error from looping forever.
const MaxRaceMonths = 600

// Fund is one goal's position in a funding race: what is left to raise, and the
// most it can absorb in a month. A zero MonthlyCapMinor means uncapped — the
// goal takes whatever the surplus offers, which is what a goal with no monthly
// plan does.
type Fund struct {
	RemainingMinor  int64
	MonthlyCapMinor int64
}

// Landing is when a goal finishes under one funding order.
type Landing struct {
	// Months is how many monthly contributions it takes. Zero means it was
	// already funded before the race started.
	Months int
	// Reached is false when the goal does not finish inside MaxRaceMonths —
	// which, with a positive surplus, means the other goal is absorbing all of
	// it. Callers must not render Months in that case; it is the cap, not a date.
	Reached bool
}

// Order is what both goals do under one choice of who goes first.
type Order struct {
	First, Second Landing
}

// Race simulates funding two goals from one monthly surplus, first taking what
// it can absorb before second sees anything. It returns when each finishes.
//
// The month is the unit because that is the unit the surplus is stated in and
// the unit every other projection in the app uses; simulating rather than
// solving in closed form is deliberate, because the handover — the month the
// first goal completes and the leftover spills to the second — is exactly the
// part a formula gets subtly wrong.
//
// A non-positive surplus reaches nothing: with no money there is no order to
// choose, and reporting a landing would be inventing one.
func Race(surplusMinor int64, first, second Fund) Order {
	out := Order{
		First:  Landing{Reached: first.RemainingMinor <= 0},
		Second: Landing{Reached: second.RemainingMinor <= 0},
	}
	if surplusMinor <= 0 {
		return out
	}
	remA, remB := first.RemainingMinor, second.RemainingMinor
	for m := 1; m <= MaxRaceMonths; m++ {
		if remA <= 0 && remB <= 0 {
			break
		}
		budget := surplusMinor
		// The first goal takes what it can absorb, capped by its monthly plan and
		// by what it still needs — a goal never takes more than it is short.
		if remA > 0 {
			take := budget
			if first.MonthlyCapMinor > 0 && first.MonthlyCapMinor < take {
				take = first.MonthlyCapMinor
			}
			if take > remA {
				take = remA
			}
			remA -= take
			budget -= take
			if remA <= 0 && !out.First.Reached {
				out.First = Landing{Months: m, Reached: true}
			}
		}
		// Whatever the first goal could not absorb THIS month spills over — that
		// spill is the whole reason a cap matters, and dropping it would make a
		// capped first goal look free.
		if remB > 0 && budget > 0 {
			take := budget
			if second.MonthlyCapMinor > 0 && second.MonthlyCapMinor < take {
				take = second.MonthlyCapMinor
			}
			if take > remB {
				take = remB
			}
			remB -= take
			if remB <= 0 && !out.Second.Reached {
				out.Second = Landing{Months: m, Reached: true}
			}
		}
	}
	return out
}

// Swap is the comparison the surface actually needs: both orderings, side by
// side, so the trade is visible rather than inferred.
type Swap struct {
	// AFirst is the outcome when goal A is funded first: AFirst.First is A's
	// landing, AFirst.Second is B's.
	AFirst Order
	// BFirst is the mirror: BFirst.First is B's landing, BFirst.Second is A's.
	BFirst Order
}

// Compare runs both funding orders over the same surplus.
func Compare(surplusMinor int64, a, b Fund) Swap {
	return Swap{
		AFirst: Race(surplusMinor, a, b),
		BFirst: Race(surplusMinor, b, a),
	}
}

// Matters reports whether the choice of order changes anything. When both goals
// land in the same months either way — one is already funded, the surplus covers
// both plans in full, or nothing is reachable at all — presenting a trade-off
// would be manufacturing a decision the user does not have.
func (s Swap) Matters() bool {
	aFirstA, aFirstB := s.AFirst.First, s.AFirst.Second
	bFirstB, bFirstA := s.BFirst.First, s.BFirst.Second
	return aFirstA != bFirstA || aFirstB != bFirstB
}
