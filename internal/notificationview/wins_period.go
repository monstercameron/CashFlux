// SPDX-License-Identifier: MIT

package notificationview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func keptMonthlyBudgetWins(budgets []domain.Budget, all budgeting.NettedLedger, start, end time.Time, rates currency.Rates) int {
	hasMonthly := false
	for _, budget := range budgets {
		if budget.Period == domain.PeriodMonthly {
			hasMonthly = true
			break
		}
	}
	if !hasMonthly {
		return 0
	}
	// All eligible budgets share this completed month. Select it once so each
	// canonical Spent fold visits K monthly rows instead of N ledger rows.
	month := budgetWinMonth(all, start, end)
	kept := 0
	for _, budget := range budgets {
		if budget.Period != domain.PeriodMonthly {
			continue
		}
		spent, err := budgeting.Spent(budget, month, start, end, rates)
		if err == nil && spent.Amount > 0 && spent.Amount <= budget.Limit.Amount {
			kept++
		}
	}
	return kept
}

// Preserve source order and netted type: transactions, owners, splits, tags,
// direction, per-row FX rounding and checked addition remain Spent's job. In
// particular, never reconstruct/refund-net this already-netted projection.
func budgetWinMonth(all budgeting.NettedLedger, start, end time.Time) budgeting.NettedLedger {
	var month budgeting.NettedLedger
	for _, txn := range all {
		if dateutil.InRange(txn.Date, start, end) {
			month = append(month, txn)
		}
	}
	return month
}
