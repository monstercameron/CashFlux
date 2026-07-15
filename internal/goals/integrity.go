// SPDX-License-Identifier: MIT

package goals

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// This file holds the earmark-integrity derivation (XC7): per account, is the
// money goals have earmarked against it actually there? Goals reserve balance via
// virtual earmarks (GoalAllocation), but nothing stops the underlying account
// from being spent down below the total reserved. When the earmarks attributed to
// an account exceed its real balance, goal money has silently been spent — this
// pure, table-driven-tested derivation surfaces that breach.

// AccountEarmark is one account's earmark-integrity position: how much is
// virtually reserved against it versus its real balance, both in minor units of
// the account's currency.
type AccountEarmark struct {
	// AccountID identifies the account.
	AccountID string
	// EarmarkedMinor is the total reserved against the account across all goals.
	EarmarkedMinor int64
	// BalanceMinor is the account's real current balance.
	BalanceMinor int64
}

// OverMinor is how much the earmarks exceed the balance — the amount of goal
// money that has been spent. Zero when the balance covers the earmarks.
func (a AccountEarmark) OverMinor() int64 {
	if a.EarmarkedMinor > a.BalanceMinor {
		return a.EarmarkedMinor - a.BalanceMinor
	}
	return 0
}

// Breached reports whether earmarks exceed the balance — goal money has been
// spent and nothing else says so.
func (a AccountEarmark) Breached() bool {
	return a.EarmarkedMinor > a.BalanceMinor
}

// EarmarkIntegrity reports the earmark-integrity position of every account that
// has at least one earmark against it. balances maps account id to its real
// balance in minor units (accounts absent from the map are treated as zero
// balance — an earmark against an unknown account is a breach). Results are
// sorted by account id for stable rendering.
func EarmarkIntegrity(goals []domain.Goal, balances map[string]int64) []AccountEarmark {
	earmarked := map[string]int64{}
	for _, g := range goals {
		for _, al := range g.Allocations {
			if al.AccountID == "" {
				continue
			}
			earmarked[al.AccountID] += al.Amount.Amount
		}
	}
	out := make([]AccountEarmark, 0, len(earmarked))
	for id, total := range earmarked {
		out = append(out, AccountEarmark{
			AccountID:      id,
			EarmarkedMinor: total,
			BalanceMinor:   balances[id],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out
}

// AccountBreached reports whether a single account is over-earmarked given the
// goals and the account's real balance in minor units.
func AccountBreached(goals []domain.Goal, accountID string, balanceMinor int64) bool {
	return AccountEarmarkedMinor(goals, accountID, "") > balanceMinor
}

// GoalSweepAllowed reports whether a sweep may earmark into a goal without
// worsening an existing breach — the XC7 gate over XC6. It answers false when any
// of the goal's linked accounts is already over-earmarked, so the sweep never
// pours more phantom reservation into an account whose goal money is already
// gone. balances maps account id to real balance in minor units. A goal with no
// linked accounts is allowed (there is no account to breach).
func GoalSweepAllowed(g domain.Goal, allGoals []domain.Goal, balances map[string]int64) bool {
	for _, acct := range g.LinkedAccountIDs() {
		if AccountBreached(allGoals, acct, balances[acct]) {
			return false
		}
	}
	return true
}
