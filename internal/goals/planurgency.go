// SPDX-License-Identifier: MIT

package goals

// PlanUrgency grades HOW FAR BEHIND a "needs a plan" goal is, refining the coarse
// Watch / At-risk verdict (see AssessHealth) so a cluster of stretch goals can be
// prioritised instead of every card reading the same "Watch". It is derived from the
// same figures AssessHealth uses: the monthly contribution the deadline requires
// (requiredMonthly), the household's free monthly cash (monthlySurplus), and the goal's
// fair share of that cash (fairShare = surplus ÷ active-deadlined-goals).
//
// It is a pure ordering/labelling read of figures already computed for the card — no
// new pace math, no mutation — so it can never disagree with the health verdict:
// everything AssessHealth calls At-risk grades FarBehind here, and a Watch goal splits
// into Watch / Slipping / FarBehind by how far over its fair share it must reach.
type PlanUrgency int

const (
	// UrgencyNone: nothing to grade — the goal is within its fair share (on track) or
	// there is no requirement to measure. Not part of the "needs a plan" group.
	UrgencyNone PlanUrgency = iota
	// UrgencyWatch: a mild stretch — the goal needs a little over its fair share of the
	// free cash. Worth watching, not yet alarming (neutral tone).
	UrgencyWatch
	// UrgencySlipping: the goal needs well over its fair share (≥ ~1.5×) but is still
	// reachable within the total free cash (warn tone).
	UrgencySlipping
	// UrgencyFarBehind: the required pace is a large multiple of the fair share (≥ ~3×)
	// OR exceeds the entire monthly surplus (unreachable even if ALL free cash went to
	// this one goal) — the strongest behind-schedule grade (danger tone).
	UrgencyFarBehind
)

// ClassifyPlanUrgency grades a behind-schedule goal from its required monthly
// contribution against the free cash available to it. All amounts are base-currency
// minor units; fairShare is the goal's fair split of the surplus (surplus ÷
// active-deadlined-goals), already computed by the caller.
//
// Grading (integer arithmetic only — money is never a float):
//
//	required ≤ 0                        → None       (nothing needed)
//	surplus ≤ 0  or  required > surplus → FarBehind  (unreachable with all slack)
//	fairShare ≤ 0                       → FarBehind  (no fair share to draw on)
//	required ≤ fairShare                → None       (within its share — on track)
//	required ≥ 3 × fairShare            → FarBehind  (needs many times its share)
//	required ≥ 1.5 × fairShare          → Slipping
//	otherwise                          → Watch
func ClassifyPlanUrgency(requiredMonthly, monthlySurplus, fairShare int64) PlanUrgency {
	if requiredMonthly <= 0 {
		return UrgencyNone
	}
	// Unreachable even if every spare dollar went to this one goal.
	if monthlySurplus <= 0 || requiredMonthly > monthlySurplus {
		return UrgencyFarBehind
	}
	// Reachable within the total slack, but the goal has no fair share to measure the
	// stretch against — treat the whole demand as far behind.
	if fairShare <= 0 {
		return UrgencyFarBehind
	}
	if requiredMonthly <= fairShare {
		return UrgencyNone
	}
	if requiredMonthly >= 3*fairShare {
		return UrgencyFarBehind
	}
	// 1.5× as integers: required ≥ 1.5·fair  ⇔  2·required ≥ 3·fair.
	if 2*requiredMonthly >= 3*fairShare {
		return UrgencySlipping
	}
	return UrgencyWatch
}

// Rank orders urgencies for the "needs a plan" list, most urgent first (lower = earlier):
// FarBehind (0) → Slipping (1) → Watch (2) → None (3). Used as a sort tiebreak so the
// cards inside the group present worst-first.
func (u PlanUrgency) Rank() int {
	switch u {
	case UrgencyFarBehind:
		return 0
	case UrgencySlipping:
		return 1
	case UrgencyWatch:
		return 2
	default:
		return 3
	}
}
