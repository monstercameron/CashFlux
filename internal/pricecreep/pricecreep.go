// SPDX-License-Identifier: MIT

// Package pricecreep holds the pure budget-impact preview for the price-creep
// accept flow (XC5): when the user accepts a higher recurring price, this shows
// what the increase does to the affected budget — the before/after usage the
// accept-flow modal renders as two short lines — and the limit bump that would
// keep the budget whole. Pure and deterministic; no syscall/js.
package pricecreep

// BudgetImpact is the before/after read of accepting a price increase against
// the recurring's budget. Money fields are minor units in Currency.
type BudgetImpact struct {
	// HasBudget is false when the recurring's category has no budget — the accept
	// flow then shows only the price change, no budget lines.
	HasBudget  bool
	BudgetName string
	Currency   string

	LimitMinor       int64 // the budget's current period limit
	SpentBeforeMinor int64 // spent so far this period
	// DeltaMonthlyMinor is the monthly-equivalent increase the new price adds.
	DeltaMonthlyMinor int64

	BeforePct int // spent / limit, rounded
	AfterPct  int // (spent + delta) / limit, rounded

	// SuggestedLimitMinor is limit + delta — the "also raise the budget?" target.
	SuggestedLimitMinor int64
}

// Preview computes the budget impact of a monthly-equivalent price increase. When
// hasBudget is false (or limitMinor is 0), the percentages are zero and only the
// price change is meaningful. Percentages round to the nearest whole percent.
func Preview(budgetName, currency string, limitMinor, spentMinor, deltaMonthlyMinor int64, hasBudget bool) BudgetImpact {
	imp := BudgetImpact{
		HasBudget: hasBudget && limitMinor > 0, BudgetName: budgetName, Currency: currency,
		LimitMinor: limitMinor, SpentBeforeMinor: spentMinor, DeltaMonthlyMinor: deltaMonthlyMinor,
	}
	if !imp.HasBudget {
		return imp
	}
	imp.BeforePct = pct(spentMinor, limitMinor)
	imp.AfterPct = pct(spentMinor+deltaMonthlyMinor, limitMinor)
	imp.SuggestedLimitMinor = limitMinor + deltaMonthlyMinor
	return imp
}

// pct returns round(100 * n / d) for a positive denominator, else 0.
func pct(n, d int64) int {
	if d <= 0 {
		return 0
	}
	// Rounded integer percentage without floats.
	return int((n*100 + d/2) / d)
}
