// SPDX-License-Identifier: MIT

// Package waterfall computes a payday funding plan: given an income amount and a
// priority-ranked list of goals with their period funding quotas, it fills each
// goal in order up to the quota still outstanding this period, cascading whatever
// remains down the list. It is the pure, ordering-and-cascade layer beneath the
// /goals "fund goals from this paycheck" preview-approve card (GL1).
//
// Priority order is the caller's — the Allocate page's rank/exclude/split is the
// intended source, so the waterfall does not re-rank; it funds in the slice order
// it is handed. Excluded goals are simply omitted by the caller.
//
// Earmark integrity (XC7) is a hard constraint: a goal is never planned past the
// real free balance of the account it earmarks against. Because several goals can
// share one funding account, the plan tracks each account's remaining free balance
// as it fills, so the combined plan never earmarks an account past its balance.
//
// Pure Go, no platform dependencies; unit-tested on native Go. Money is in integer
// minor units of a single (household base) currency — the caller converts first.
package waterfall

// GoalQuota describes one goal's position in the funding waterfall.
type GoalQuota struct {
	// GoalID identifies the goal the plan line funds.
	GoalID string
	// Name is the goal's display name, carried through for the preview card.
	Name string
	// QuotaMinor is the goal's funding target for this period (e.g. its monthly
	// contribution / MonthlyNeeded pace). Non-positive quotas are skipped.
	QuotaMinor int64
	// AlreadyFundedMinor is how much of the quota is already met this period, so a
	// partially-funded goal only draws the remainder. Clamped at the quota.
	AlreadyFundedMinor int64
	// AccountID is the account this goal earmarks against (XC7). Empty means the
	// goal has no funding account and is skipped (nothing to earmark).
	AccountID string
}

// AccountFree maps an account id to its real free balance in minor units — the
// account's balance minus what OTHER goals already earmark against it. It is the
// XC7 ceiling: the waterfall never plans an account past this figure.
type AccountFree map[string]int64

// PlanLine is one goal's funded amount in the waterfall plan.
type PlanLine struct {
	GoalID    string
	Name      string
	AccountID string
	// AmountMinor is the amount to earmark toward this goal from the income.
	AmountMinor int64
}

// Plan is the outcome of a waterfall: the per-goal funding lines (in priority
// order, only goals that received funding) and the income left over after every
// reachable quota was met or the money ran out.
type Plan struct {
	Lines          []PlanLine
	RemainderMinor int64
	// FundedMinor is the total planned across all lines (income - remainder).
	FundedMinor int64
}

// Compute builds the funding plan for incomeMinor across the priority-ordered
// goals. For each goal in order it funds the smaller of (a) the quota still
// outstanding this period, (b) the income still unallocated, and (c) the account's
// remaining free balance (XC7). Accounts shared by several goals are debited once
// as the plan fills, so the combined plan respects every account's real balance.
//
// Goals with a non-positive outstanding quota, no funding account, or an account
// with no free balance are skipped. The remainder is whatever income is left after
// the last fundable goal.
func Compute(incomeMinor int64, goalsQ []GoalQuota, free AccountFree) Plan {
	plan := Plan{RemainderMinor: incomeMinor}
	if incomeMinor <= 0 {
		return plan
	}
	remaining := incomeMinor

	// Working copy of each account's free balance so shared accounts deplete.
	acctFree := make(map[string]int64, len(free))
	for id, f := range free {
		acctFree[id] = f
	}

	for _, gq := range goalsQ {
		if remaining <= 0 {
			break
		}
		if gq.AccountID == "" || gq.QuotaMinor <= 0 {
			continue
		}
		outstanding := gq.QuotaMinor - gq.AlreadyFundedMinor
		if outstanding <= 0 {
			continue
		}
		af := acctFree[gq.AccountID]
		if af <= 0 {
			continue
		}
		give := outstanding
		if give > remaining {
			give = remaining
		}
		if give > af {
			give = af
		}
		if give <= 0 {
			continue
		}
		plan.Lines = append(plan.Lines, PlanLine{
			GoalID:      gq.GoalID,
			Name:        gq.Name,
			AccountID:   gq.AccountID,
			AmountMinor: give,
		})
		remaining -= give
		acctFree[gq.AccountID] = af - give
		plan.FundedMinor += give
	}
	plan.RemainderMinor = remaining
	return plan
}
