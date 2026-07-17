// SPDX-License-Identifier: MIT

package goals

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// This file holds two additive goal concerns, both pure and unit-tested:
//
//   - Virtual allocation ("earmarks"): reserving amounts of accounts' existing balances
//     for a goal WITHOUT posting a transaction. Coverage = committed savings + earmarks.
//   - Review staleness: flagging a goal for review once its ReviewCadence has elapsed
//     since it was last touched (LastReviewedAt).

// AllocatedTotal returns the sum of a goal's virtual earmarks as money in the goal's
// target currency (allocations are stored in that currency by the allocate UI).
func AllocatedTotal(g domain.Goal) money.Money {
	cur := g.TargetAmount.Currency
	if cur == "" {
		cur = g.CurrentAmount.Currency
	}
	return money.New(g.AllocatedMinor(), cur)
}

// CoverageMinor is committed savings (CurrentAmount) plus virtual earmarks, in minor
// units of the goal's target currency. It answers "how much of the target is already
// accounted for, whether moved or merely reserved?".
func CoverageMinor(g domain.Goal) int64 {
	return g.CurrentAmount.Amount + g.AllocatedMinor()
}

// CoveredRemaining is the gap between the target and what's already accounted
// for — saved (CurrentAmount) PLUS earmarked — floored at zero, in the target
// currency. This is the remaining figure every pace computation amortizes:
// earmarks are first-class progress, so money merely reserved (not yet moved)
// still shrinks what's left to find. Like Remaining, a saved/target currency
// mismatch is an error (allocations are stored in the target currency).
func CoveredRemaining(g domain.Goal) (money.Money, error) {
	rem, err := g.TargetAmount.Sub(g.CurrentAmount)
	if err != nil {
		return money.Money{}, err
	}
	rem.Amount -= g.AllocatedMinor()
	if rem.Amount < 0 {
		rem.Amount = 0
	}
	return rem, nil
}

// CoveragePercent is CoverageMinor over the target, clamped to 0..100. A goal with no
// positive target (non-financial, or unset) reports 0.
func CoveragePercent(g domain.Goal) int {
	t := g.TargetAmount.Amount
	if t <= 0 {
		return 0
	}
	p := int(CoverageMinor(g) * 100 / t)
	switch {
	case p < 0:
		return 0
	case p > 100:
		return 100
	default:
		return p
	}
}

// AccountEarmarkedMinor sums how much is virtually earmarked against accountID across
// the given goals, in minor units. excludeGoalID (optional) skips one goal — the editor
// passes the goal being edited so its own existing earmarks don't count against its new
// budget. Minor units are summed directly, which is exact for a single-currency household
// (the common case) and a safe approximation otherwise.
func AccountEarmarkedMinor(goals []domain.Goal, accountID, excludeGoalID string) int64 {
	var sum int64
	for _, g := range goals {
		if g.ID == excludeGoalID {
			continue
		}
		for _, a := range g.Allocations {
			if a.AccountID == accountID {
				sum += a.Amount.Amount
			}
		}
	}
	return sum
}

// AvailableToEarmarkMinor is how much of an account's balance is still free to earmark
// for a goal: the balance minus what OTHER goals already reserve against it. Never
// negative. balanceMinor is the account's current balance in the goal's currency (the
// caller converts if needed). excludeGoalID is the goal being edited.
func AvailableToEarmarkMinor(goals []domain.Goal, accountID string, balanceMinor int64, excludeGoalID string) int64 {
	free := balanceMinor - AccountEarmarkedMinor(goals, accountID, excludeGoalID)
	if free < 0 {
		return 0
	}
	return free
}

// EarmarkStatus is the coarse, glanceable state of a goal's virtual allocation — the
// basis for the card's earmark badge and a first-class value the formula/assessment layer
// can key off (exposed numerically as goal_<slug>_covered_pct).
type EarmarkStatus string

const (
	// EarmarkNone means nothing is virtually reserved for the goal yet.
	EarmarkNone EarmarkStatus = "none"
	// EarmarkPartial means some balance is earmarked but committed + earmarked still
	// falls short of the target.
	EarmarkPartial EarmarkStatus = "partial"
	// EarmarkFull means committed savings + earmarks cover (or exceed) the target — the
	// goal is fully funded in place, whether or not any money has actually moved.
	EarmarkFull EarmarkStatus = "full"
)

// EarmarkOf classifies a goal's earmark coverage. A goal with no positive target (non-
// financial, or unset) is always EarmarkNone — earmarking is a financial-goal concept.
func EarmarkOf(g domain.Goal) EarmarkStatus {
	if g.TargetAmount.Amount <= 0 || g.AllocatedMinor() <= 0 {
		return EarmarkNone
	}
	if CoverageMinor(g) >= g.TargetAmount.Amount {
		return EarmarkFull
	}
	return EarmarkPartial
}

// ReviewDue reports whether a goal is due for review at time now: it has a ReviewCadence
// and that cadence has elapsed since LastReviewedAt. A cadence with no recorded review
// (zero LastReviewedAt) is due immediately; no cadence never nags.
func ReviewDue(g domain.Goal, now time.Time) bool {
	if g.ReviewCadence == "" {
		return false
	}
	if g.LastReviewedAt.IsZero() {
		return true
	}
	return now.After(g.LastReviewedAt.Add(reviewCadenceDuration(g.ReviewCadence)))
}

// reviewCadenceDuration is the approximate span of one review cadence step. Month-based
// steps use round day counts (the check only needs day granularity); an unknown cadence
// falls back to monthly.
func reviewCadenceDuration(c domain.RecurringCadence) time.Duration {
	const day = 24 * time.Hour
	switch c {
	case domain.CadenceDaily:
		return day
	case domain.CadenceWeekly:
		return 7 * day
	case domain.CadenceBiweekly:
		return 14 * day
	case domain.CadenceSemimonthly:
		return 15 * day
	case domain.CadenceMonthly:
		return 30 * day
	case domain.CadenceQuarterly:
		return 91 * day
	case domain.CadenceYearly:
		return 365 * day
	default:
		return 30 * day
	}
}
