// SPDX-License-Identifier: MIT

package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// IncomeForBudgets resolves the monthly income figure that budgeting helpers
// (50/30/20 split, safe-to-spend, envelope fill) should use as their base.
//
// Precedence rule — configured income wins:
//
//	If configuredMinor > 0 the caller's stated expected monthly income is
//	returned unchanged. This lets the user anchor the budget to their
//	take-home pay even in months where actual deposits do not match
//	(e.g. a new job, a partial first month, or a pay-advance).
//
// Actual income fallback:
//
//	When configuredMinor == 0 (or negative) the function sums all income
//	transactions in the half-open window [start, end), converting each one
//	to base currency via the supplied FX rate table. Transfer transactions
//	are excluded automatically because domain.Transaction.IsIncome() requires
//	IsTransfer() == false and a positive amount.
//
// Income is summed inline (domain.Transaction.IsIncome + currency.ConvertBetween
// over the same dateutil.InRange window ledger.PeriodTotals uses) rather than
// calling ledger.PeriodTotals on purpose: budgeting must NOT import ledger,
// because ledger's own tests import budgeting and the dependency would form an
// import cycle in ledger's test build.
//
// On an FX conversion failure (e.g. a missing rate) the function returns 0 so
// callers can surface a "configure your income" prompt rather than silently
// computing a wrong budget. The returned value is in minor units of base.
func IncomeForBudgets(
	configuredMinor int64,
	txns []domain.Transaction,
	start, end time.Time,
	base string,
	rates currency.Rates,
) int64 {
	if configuredMinor > 0 {
		return configuredMinor
	}

	var total int64
	for _, t := range txns {
		if !t.IsIncome() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := currency.ConvertBetween(t.Amount.Amount, t.Amount.Currency, base, rates)
		if err != nil {
			return 0
		}
		total += conv
	}
	return total
}
