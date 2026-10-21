// SPDX-License-Identifier: MIT

package afford

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// CanAffordChecked preserves CanAfford's constant-monthly-flow model while
// rejecting unrepresentable money or month counts. Negative horizons mean now;
// the purchase and reserve must be nonnegative. An error is an unavailable
// projection, never an unaffordable or affordable result.
func CanAffordChecked(amount, start, monthlyNet int64, months int, reserved int64) (Result, error) {
	if amount < 0 || reserved < 0 {
		return Result{}, fmt.Errorf("afford: amount and reserve must be nonnegative")
	}
	if months < 0 {
		months = 0
	}
	change, err := money.Mul(monthlyNet, int64(months))
	if err != nil {
		return Result{}, fmt.Errorf("afford: projected cash flow: %w", err)
	}
	projected, err := money.Add(start, change)
	if err != nil {
		return Result{}, fmt.Errorf("afford: projected balance: %w", err)
	}
	available, err := money.Sub(projected, reserved)
	if err != nil {
		return Result{}, fmt.Errorf("afford: available balance: %w", err)
	}
	result := Result{Affordable: available >= amount, ProjectedBalance: projected, Available: available}
	if !result.Affordable {
		result.Shortfall, err = money.Sub(amount, available)
		if err != nil {
			return Result{}, fmt.Errorf("afford: shortfall: %w", err)
		}
	}
	// This order keeps an already affordable purchase valid even when an unused
	// amount+reserve intermediate would overflow.
	currentAvailable, err := money.Sub(start, reserved)
	if err != nil {
		return Result{}, fmt.Errorf("afford: current available balance: %w", err)
	}
	if currentAvailable >= amount {
		return result, nil
	}
	if monthlyNet <= 0 {
		result.MonthsNeeded = -1
		return result, nil
	}
	need, err := money.Sub(amount, currentAvailable)
	if err != nil {
		return Result{}, fmt.Errorf("afford: amount needed: %w", err)
	}
	needed := need / monthlyNet
	if need%monthlyNet != 0 {
		needed++
	}
	if needed > int64(^uint(0)>>1) {
		return Result{}, fmt.Errorf("afford: months needed does not fit int: %w", money.ErrOverflow)
	}
	result.MonthsNeeded = int(needed)
	return result, nil
}
