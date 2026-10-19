// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	return a.leftoverSweepHeadroom(target, all) > 0
}

// LeftoverSweepCapacity returns the most that can be earmarked from cur without
// reserving more than the linked account's remaining balance.
func (a *App) LeftoverSweepCapacity(goalID, cur string) (int64, error) {
	if cur == "" {
		return 0, fmt.Errorf("appstate: leftover sweep: currency is required")
	}
	var target domain.Goal
	all := a.Goals()
	for _, goal := range all {
		if goal.ID == goalID {
			target = goal
			break
		}
	}
	if target.ID == "" {
		return 0, fmt.Errorf("appstate: leftover sweep: goal %q not found", goalID)
	}
	linked := target.LinkedAccountIDs()
	if len(linked) == 0 {
		return 0, fmt.Errorf("appstate: leftover sweep: goal %q has no linked account", goalID)
	}
	account, ok := findAccount(a, linked[0])
	if !ok {
		return 0, fmt.Errorf("appstate: leftover sweep: linked account %q not found", linked[0])
	}
	headroom := a.leftoverSweepHeadroom(target, all)
	if headroom <= 0 {
		return 0, nil
	}
	rates := currency.Rates{Base: a.baseCurrency(), Rates: a.Settings().FXRates}
	return maxSweepCapacityMinor(headroom, account.Currency, cur, rates)
}

func maxSweepCapacityMinor(headroom int64, accountCurrency, sweepCurrency string, rates currency.Rates) (int64, error) {
	if headroom <= 0 {
		return 0, nil
	}
	if accountCurrency == "" || sweepCurrency == "" {
		return 0, fmt.Errorf("currency is required")
	}
	if accountCurrency == sweepCurrency {
		return headroom, nil
	}
	upper, err := rates.Convert(money.New(headroom, accountCurrency), sweepCurrency)
	if err != nil {
		return 0, fmt.Errorf("convert account headroom: %w", err)
	}
	lo, hi := int64(0), upper.Amount
	for lo < hi {
		distance := hi - lo
		mid := lo + distance/2 + distance%2
		converted, convertErr := rates.Convert(money.New(mid, sweepCurrency), accountCurrency)
		if convertErr != nil {
			return 0, fmt.Errorf("check sweep capacity: %w", convertErr)
		}
		if converted.Amount <= headroom {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, nil
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
	if strings.TrimSpace(cur) == "" {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: currency is required")
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
	acctID := linked[0]
	base := a.baseCurrency()
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}
	acct, ok := findAccount(a, acctID)
	if !ok {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: linked account %q not found", acctID)
	}
	accountCurrency := acct.Currency
	if accountCurrency == "" {
		accountCurrency = cur
	}
	sweepInAccountCurrency, err := rates.Convert(money.New(totalMinor, cur), accountCurrency)
	if err != nil {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: convert to linked account currency: %w", err)
	}
	headroom := a.leftoverSweepHeadroom(target, all)
	if sweepInAccountCurrency.Amount > headroom {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: amount exceeds linked account headroom")
	}
	// Merge into an existing earmark against the same account, or append a new one.
	target.Allocations = append([]domain.GoalAllocation(nil), target.Allocations...)
	merged := false
	for i, al := range target.Allocations {
		if al.AccountID == acctID {
			amount, err := mergeSweepAmount(al.Amount, totalMinor, cur, rates)
			if err != nil {
				return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: merge allocation: %w", err)
			}
			target.Allocations[i].Amount = amount
			merged = true
			break
		}
	}
	if !merged {
		amount, err := rates.Convert(money.New(totalMinor, cur), accountCurrency)
		if err != nil {
			return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: convert new allocation: %w", err)
		}
		target.Allocations = append(target.Allocations, domain.GoalAllocation{
			AccountID: acctID,
			Amount:    amount,
		})
	}

	if err := a.PutGoal(target); err != nil {
		return domain.Goal{}, fmt.Errorf("appstate: leftover sweep: save goal: %w", err)
	}
	a.log.Info("leftover sweep earmarked to goal", "goal", goalID, "account", acctID, "amount", totalMinor)
	return target, nil
}

// leftoverSweepHeadroom returns the first linked account's remaining earmark
// capacity in that account's minor units. Conversion errors make the sweep fail
// closed by returning zero.
func (a *App) leftoverSweepHeadroom(target domain.Goal, all []domain.Goal) int64 {
	linked := target.LinkedAccountIDs()
	if len(linked) == 0 {
		return 0
	}
	acct, ok := findAccount(a, linked[0])
	if !ok {
		return 0
	}
	cur := acct.Currency
	if cur == "" {
		return 0
	}
	rates := currency.Rates{Base: a.baseCurrency(), Rates: a.Settings().FXRates}
	balances := a.linkedAccountBalances(all)
	var earmarked int64
	for _, goal := range all {
		for _, allocation := range goal.Allocations {
			if allocation.AccountID != acct.ID {
				continue
			}
			amount := allocation.Amount
			if amount.Currency == "" {
				amount.Currency = cur
			}
			converted, err := rates.Convert(amount, cur)
			if err != nil {
				return 0
			}
			var errAdd error
			earmarked, errAdd = addMinorChecked(earmarked, converted.Amount)
			if errAdd != nil {
				return 0
			}
		}
	}
	balance := balances[acct.ID]
	if balance <= earmarked {
		return 0
	}
	return balance - earmarked
}

func addMinorChecked(a, b int64) (int64, error) {
	if b > 0 && a > int64(^uint64(0)>>1)-b || b < 0 && a < -int64(^uint64(0)>>1)-1-b {
		return 0, fmt.Errorf("minor-unit amount overflow")
	}
	return a + b, nil
}

func mergeSweepAmount(allocation money.Money, totalMinor int64, sweepCurrency string, rates currency.Rates) (money.Money, error) {
	allocationCurrency := allocation.Currency
	if allocationCurrency == "" {
		allocationCurrency = sweepCurrency
	}
	sweepAmount, err := rates.Convert(money.New(totalMinor, sweepCurrency), allocationCurrency)
	if err != nil {
		return money.Money{}, fmt.Errorf("convert amount to allocation currency: %w", err)
	}
	if allocation.Currency == "" {
		allocation.Currency = allocationCurrency
	}
	merged, err := allocation.Add(sweepAmount)
	if err != nil {
		return money.Money{}, fmt.Errorf("add to allocation: %w", err)
	}
	return merged, nil
}
