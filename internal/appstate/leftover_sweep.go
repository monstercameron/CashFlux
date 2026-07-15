// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// This file implements the write side of the leftover-sweep ritual (XC6): one
// approval earmarks a month's swept budget leftovers toward a chosen goal, by
// merging a virtual GoalAllocation against the goal's linked account. It composes
// the earmark-integrity gate (XC7) so a sweep never pours phantom reservation
// into an account whose goal money has already been spent. No syscall/js — the
// wasm layer calls this after computing the sweep total, then persists.

// linkedAccountBalances returns the real balance in minor units of every account
// linked to any of the given goals, for the earmark-integrity gate.
func (a *App) linkedAccountBalances(goalList []domain.Goal) map[string]int64 {
	bals := map[string]int64{}
	for _, g := range goalList {
		for _, acctID := range g.LinkedAccountIDs() {
			if _, seen := bals[acctID]; seen {
				continue
			}
			acc, ok := findAccount(a, acctID)
			if !ok {
				bals[acctID] = 0
				continue
			}
			bal, err := ledger.Balance(acc, a.Transactions())
			if err != nil {
				a.log.Warn("leftover sweep: balance lookup failed", "account", acctID, "err", err)
				bals[acctID] = 0
				continue
			}
			bals[acctID] = bal.Amount
		}
	}
	return bals
}

// SweepAllowedForGoal reports whether the target goal currently passes the
// earmark-integrity gate (XC7): none of its linked accounts is over-earmarked. It
// is the check the sweep card runs before offering its primary action, and the
// same predicate fed into budgeting.ComputeSweep as goalAllowed.
func (a *App) SweepAllowedForGoal(goalID string) bool {
	all := a.Goals()
	var target domain.Goal
	for _, g := range all {
		if g.ID == goalID {
			target = g
			break
		}
	}
	if target.ID == "" {
		return false
	}
	return goals.GoalSweepAllowed(target, all, a.linkedAccountBalances(all))
}

// ApplyLeftoverSweep earmarks totalMinor (in cur) of swept budget leftovers toward
// the goal identified by goalID, by merging a virtual GoalAllocation against the
// goal's first linked account. It enforces the XC7 gate: the sweep is refused when
// the goal's linked account is already over-earmarked. It returns the affected
// goal on success so the caller can toast/undo. The caller persists (RequestPersist).
func (a *App) ApplyLeftoverSweep(goalID string, totalMinor int64, cur string) (domain.Goal, error) {
	if totalMinor <= 0 {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: amount must be positive")
	}
	all := a.Goals()
	var target domain.Goal
	for _, g := range all {
		if g.ID == goalID {
			target = g
			break
		}
	}
	if target.ID == "" {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: goal %q not found", goalID)
	}
	linked := target.LinkedAccountIDs()
	if len(linked) == 0 {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: goal %q has no linked account to earmark against", goalID)
	}
	if !goals.GoalSweepAllowed(target, all, a.linkedAccountBalances(all)) {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: goal %q linked account is over-earmarked", goalID)
	}

	acctID := linked[0]
	// Merge into an existing earmark against the same account, or append a new one.
	merged := false
	for i, al := range target.Allocations {
		if al.AccountID == acctID {
			target.Allocations[i].Amount = money.New(al.Amount.Amount+totalMinor, al.Amount.Currency)
			merged = true
			break
		}
	}
	if !merged {
		target.Allocations = append(target.Allocations, domain.GoalAllocation{
			AccountID: acctID,
			Amount:    money.New(totalMinor, cur),
		})
	}

	if err := a.PutGoal(target); err != nil {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: save goal: %w", err)
	}
	a.log.Info("leftover sweep earmarked to goal", "goal", goalID, "account", acctID, "amount", totalMinor)
	return target, nil
}
