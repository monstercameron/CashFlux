// SPDX-License-Identifier: MIT

package goals

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// ─── C401: the date a goal could actually be met by ──────────────────────────
//
// A goal on the "Needs a plan" list is there because its deadline needs more
// money per month than the household has. There are only two honest ways out:
// find more money, or move the date. The app cannot do the first, so the one
// action it can offer in a click is the second — and offering it means being
// able to say WHICH date, not just "later".
//
// The date is computed from the goal's FAIR SHARE of free cash, not the whole
// surplus. Solving one goal by silently assuming every other goal gets nothing
// would replace one infeasible plan with a set of them.

// maxWorkableMonths caps how far out a suggestion will reach. Past this the
// answer is not a date, it is "this goal needs a different plan" — and a
// suggestion of 2061 is worse than no suggestion, because it looks like advice.
const maxWorkableMonths = 600 // 50 years

// WorkableDate returns the earliest month-end by which a goal could be met at
// the given monthly contribution, and whether such a date exists at all.
//
// remainingMinor is what is still needed and monthlyMinor the contribution the
// household could realistically make toward THIS goal. A non-positive
// contribution has no answer: no date is reachable by saving nothing, and
// returning some far-future date would dress that up as a plan.
func WorkableDate(remainingMinor, monthlyMinor int64, from time.Time) (time.Time, bool) {
	if remainingMinor <= 0 {
		return dateutil.MonthStart(from), true // already there
	}
	if monthlyMinor <= 0 {
		return time.Time{}, false
	}
	months := int((remainingMinor + monthlyMinor - 1) / monthlyMinor) // ceil
	if months > maxWorkableMonths {
		return time.Time{}, false
	}
	return dateutil.AddMonths(dateutil.MonthStart(from), months), true
}

// WorkableTargetDate is WorkableDate for a whole goal, using its fair share of
// the household's free monthly cash.
//
// surplusMinor is the household's free cash per month and activeDeadlinedGoals
// how many goals share it — the same fair-share split AssessHealth uses, so the
// date this offers is consistent with the verdict that prompted it. A goal the
// app calls At risk cannot be handed a date computed as though it were the only
// goal in the house.
func WorkableTargetDate(goal domain.Goal, surplusMinor int64, activeDeadlinedGoals int, from time.Time) (time.Time, bool) {
	n := activeDeadlinedGoals
	if n < 1 {
		n = 1
	}
	fair := surplusMinor / int64(n)
	remaining := goal.TargetAmount.Amount - goal.CurrentAmount.Amount
	return WorkableDate(remaining, fair, from)
}

// FairShareCount recovers how many goals a surplus was split across, from the
// surplus and one goal's fair share.
//
// The pace verdict already did that division; re-deriving the count here rather
// than threading it through every caller keeps the retarget suggestion using the
// SAME split the verdict used. A zero or larger-than-surplus share means the
// split is unknown, which reads as "this goal alone".
func FairShareCount(surplusMinor, fairMinor int64) int {
	if fairMinor <= 0 || surplusMinor <= 0 || fairMinor > surplusMinor {
		return 1
	}
	n := int(surplusMinor / fairMinor)
	if n < 1 {
		return 1
	}
	return n
}
