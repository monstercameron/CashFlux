// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// ScaleAutoBudgetMinor applies a supported 0..200 percentage, truncating toward
// zero in minor units. Only the final result must fit int64, not the product.
func ScaleAutoBudgetMinor(monthly int64, percentage int) (int64, error) {
	if percentage < 0 || percentage > 200 {
		return 0, fmt.Errorf("auto-budget percentage must be between 0 and 200")
	}
	amount, err := money.MulDiv(monthly, int64(percentage), 100, money.RoundTowardZero)
	if err != nil {
		return 0, fmt.Errorf("scale auto-budget amount: %w", err)
	}
	return amount, nil
}

// AutoBudgetTunedRow is one prepared row in suggestion order. MonthlyMinor is
// available only when Err is nil; selection does not affect row availability.
type AutoBudgetTunedRow struct {
	Suggestion   BudgetSuggestion
	Percentage   int
	Selected     bool
	MonthlyMinor int64
	Err          error
}

// AutoBudgetTuning shares checked amounts between rows, preview and saving.
// Err invalidates TotalMinor and PendingLimits, without hiding independent rows.
// A selected zero row still counts, but only positive amounts enter pending writes.
type AutoBudgetTuning struct {
	Rows                      []AutoBudgetTunedRow
	SelectedCount, TunedCount int
	TotalMinor                int64
	PendingLimits             map[string]int64
	Err                       error
}

// PrepareAutoBudgetTuning snapshots suggestions and current selections without
// mutating inputs. Missing percentages use the same 100 default as newly learned
// suggestions; callers use FillAutoBudgetSelection for existing-budget defaults.
func PrepareAutoBudgetTuning(suggestions []BudgetSuggestion, percentages map[string]int, selected map[string]bool) AutoBudgetTuning {
	percentages = FillAutoBudgetPercentages(percentages, suggestions)
	out := AutoBudgetTuning{Rows: make([]AutoBudgetTunedRow, 0, len(suggestions)), PendingLimits: make(map[string]int64)}
	for _, suggestion := range suggestions {
		row := AutoBudgetTunedRow{Suggestion: suggestion, Percentage: percentages[suggestion.CategoryID], Selected: selected[suggestion.CategoryID]}
		if row.Percentage != 100 {
			out.TunedCount++
		}
		row.MonthlyMinor, row.Err = ScaleAutoBudgetMinor(suggestion.MonthlyMinor, row.Percentage)
		out.Rows = append(out.Rows, row)
		if !row.Selected {
			continue
		}
		out.SelectedCount++
		if out.Err != nil {
			continue
		}
		if row.Err != nil {
			out.Err = fmt.Errorf("auto-budget category %q: %w", suggestion.CategoryID, row.Err)
			continue
		}
		var err error
		out.TotalMinor, err = money.Add(out.TotalMinor, row.MonthlyMinor)
		if err != nil {
			out.Err = fmt.Errorf("sum selected auto-budget amounts: %w", err)
			continue
		}
		if row.MonthlyMinor > 0 {
			out.PendingLimits[suggestion.CategoryID] = row.MonthlyMinor
		}
	}
	if out.Err != nil {
		out.TotalMinor, out.PendingLimits = 0, nil
	}
	return out
}
