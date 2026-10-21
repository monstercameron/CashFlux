// SPDX-License-Identifier: MIT

package safespend

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// ComputeChecked preserves Compute's commitment clamping and fields while
// returning an error if subtracting commitments from liquid cash overflows.
func ComputeChecked(liquidCash, billsDue, goalContributions, committedBudgets int64, currency string) (Breakdown, error) {
	billsDue = clampNonNeg(billsDue)
	goalContributions = clampNonNeg(goalContributions)
	committedBudgets = clampNonNeg(committedBudgets)
	safe, err := money.Sub(liquidCash, billsDue)
	if err != nil {
		return Breakdown{}, fmt.Errorf("safespend: liquid cash minus bills: %w", err)
	}
	safe, err = money.Sub(safe, goalContributions)
	if err != nil {
		return Breakdown{}, fmt.Errorf("safespend: after goal contributions: %w", err)
	}
	safe, err = money.Sub(safe, committedBudgets)
	if err != nil {
		return Breakdown{}, fmt.Errorf("safespend: after committed budgets: %w", err)
	}
	return Breakdown{
		LiquidCash: liquidCash, BillsDue: billsDue, GoalContributions: goalContributions,
		CommittedBudgets: committedBudgets, SafeToSpend: safe, IsNegative: safe < 0, Currency: currency,
	}, nil
}
