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

// AveragedIncome resolves the income basis averaged over the last `months` full months
// ending at monthStart, so irregular income is budgeted off a steadier figure than one
// month alone. It sums the chosen basis (ZeroBasedIncome) over [monthStart-months,
// monthStart) and divides by months. months < 1 is treated as 1 (last month only). The
// fixed basis is a set monthly figure, so it is returned as-is (never averaged).
func AveragedIncome(
	mode string,
	paycheckMinMinor, configuredMinor int64,
	categoryIDs []string,
	txns []domain.Transaction,
	monthStart time.Time,
	months int,
	base string,
	rates currency.Rates,
) int64 {
	if months < 1 {
		months = 1
	}
	if mode == IncomeModeFixed {
		if configuredMinor > 0 {
			return configuredMinor
		}
		return 0
	}
	start := dateutil.AddMonths(monthStart, -months)
	sum := ZeroBasedIncome(mode, paycheckMinMinor, configuredMinor, categoryIDs, txns, start, monthStart, base, rates)
	return sum / int64(months)
}

// Zero-based income modes: how the "income to assign" figure is derived for a
// zero-based budget.
const (
	IncomeModeAll        = "all"        // every income deposit in the window (default)
	IncomeModePaychecks  = "paychecks"  // only deposits at/above the paycheck threshold (ignore side income)
	IncomeModeFixed      = "fixed"      // a configured monthly figure, regardless of actual deposits
	IncomeModeCategories = "categories" // only income in a chosen set of categories (pick sources by name)
)

// ZeroBasedIncome resolves the income a zero-based budget assigns against, per the
// household's chosen basis so a user can be strict (paychecks only) or loose (all
// income, some months run over):
//   - IncomeModeFixed: the configured monthly take-home (configuredMinor); 0 when unset.
//   - IncomeModePaychecks: actual income in [start,end) but only deposits whose
//     base-currency amount is >= paycheckMinMinor, so regular paychecks count and
//     small side-hustle deposits are ignored (no threshold set → same as all).
//   - IncomeModeCategories: actual income in [start,end) but only from the categories
//     in categoryIDs, so a household picks its income sources by name (e.g. count
//     Salary, hold aside Freelance). An empty categoryIDs set means nothing is chosen
//     yet, so the figure is 0 until a source is picked.
//   - IncomeModeAll (default / unknown): every income deposit in [start,end).
//
// Actual-income modes sum inline (IsIncome + ConvertBetween over the same window
// IncomeForBudgets uses) — budgeting must not import ledger. Unlike IncomeForBudgets,
// an FX failure on one deposit skips just that deposit rather than zeroing the whole
// figure, so a single missing rate can't blank the budget. Income is bucketed by the
// transaction's own CategoryID (splits are not decomposed), matching how the rest of
// the budgeting income helpers and reports.IncomeByCategory attribute income.
func ZeroBasedIncome(
	mode string,
	paycheckMinMinor, configuredMinor int64,
	categoryIDs []string,
	txns []domain.Transaction,
	start, end time.Time,
	base string,
	rates currency.Rates,
) int64 {
	if mode == IncomeModeFixed {
		if configuredMinor > 0 {
			return configuredMinor
		}
		return 0
	}
	var allowed map[string]bool
	if mode == IncomeModeCategories {
		allowed = make(map[string]bool, len(categoryIDs))
		for _, id := range categoryIDs {
			allowed[id] = true
		}
		if len(allowed) == 0 {
			return 0 // no income source chosen yet
		}
	}
	var total int64
	for _, t := range txns {
		if !t.IsIncome() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		if mode == IncomeModeCategories && !allowed[t.CategoryID] {
			continue // this income source is held aside
		}
		conv, err := currency.ConvertBetween(t.Amount.Amount, t.Amount.Currency, base, rates)
		if err != nil {
			continue // skip an unconvertible deposit, don't blank the whole figure
		}
		if mode == IncomeModePaychecks && paycheckMinMinor > 0 && conv < paycheckMinMinor {
			continue // below the paycheck threshold — treat as side income
		}
		total += conv
	}
	return total
}
