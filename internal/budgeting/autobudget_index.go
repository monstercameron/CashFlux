// SPDX-License-Identifier: MIT

package budgeting

import "github.com/monstercameron/CashFlux/internal/domain"

// MonthlyBudgetsByCategory returns the last monthly budget for each category.
// Non-monthly budgets are omitted because auto-budget only creates monthly plans.
func MonthlyBudgetsByCategory(budgets []domain.Budget) map[string]domain.Budget {
	byCategory := make(map[string]domain.Budget)
	for _, budget := range budgets {
		if budget.Period == domain.PeriodMonthly {
			byCategory[budget.CategoryID] = budget
		}
	}
	return byCategory
}
