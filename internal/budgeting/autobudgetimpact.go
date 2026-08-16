// SPDX-License-Identifier: MIT

package budgeting

import "github.com/monstercameron/CashFlux/internal/domain"

// AutoBudgetImpact is what an Auto-budget run would do to the household's plan
// as a whole (C593).
//
// The review list showed a per-category suggestion, a slider, and a total of the
// ticked rows. None of that answers the question the screen exists to answer —
// can I afford this? — because the total is of the CHANGES, not of the plan they
// land in: a suggestion that replaces an existing budget changes the household's
// total by the difference, not by its own size. That arithmetic belongs here,
// where it can be tested, rather than in a modal.
//
// Only MONTHLY budgets take part, matching what Auto budget writes; a weekly or
// yearly limit is not a monthly figure and folding it in would produce a total
// that means nothing.
type AutoBudgetImpact struct {
	// BudgetedBefore / BudgetedAfter are the household's total monthly budgeted
	// amount, in minor units of the base currency, without and with the run.
	BudgetedBefore int64
	BudgetedAfter  int64
	// Income is the monthly income the plan is measured against; 0 when no basis
	// is known, in which case the Left figures are not meaningful.
	Income int64
}

// Change is the household's total change in monthly budgeted amount.
func (i AutoBudgetImpact) Change() int64 { return i.BudgetedAfter - i.BudgetedBefore }

// LeftBefore / LeftAfter are income minus what is budgeted, before and after —
// negative when the plan asks for more than the household earns.
func (i AutoBudgetImpact) LeftBefore() int64 { return i.Income - i.BudgetedBefore }
func (i AutoBudgetImpact) LeftAfter() int64  { return i.Income - i.BudgetedAfter }

// HasIncome reports whether an income basis is known, so the UI can show the
// left-over figures rather than implying zero income.
func (i AutoBudgetImpact) HasIncome() bool { return i.Income > 0 }

// AutoBudgetImpactOf computes the plan-level effect of setting each category in
// `limits` to the given monthly amount.
//
// A category that already has a monthly budget has that budget's limit REPLACED
// (the run edits it); a category without one adds a new budget. Categories
// absent from limits are untouched. Amounts are minor units in the base
// currency — the caller converts, as Auto budget already does when it writes.
func AutoBudgetImpactOf(budgets []domain.Budget, income int64, limits map[string]int64) AutoBudgetImpact {
	out := AutoBudgetImpact{Income: income}
	replaced := map[string]bool{}
	for _, b := range budgets {
		if b.Period != domain.PeriodMonthly {
			continue
		}
		out.BudgetedBefore += b.Limit.Amount
		if next, ok := limits[b.CategoryID]; ok && !replaced[b.CategoryID] {
			replaced[b.CategoryID] = true
			out.BudgetedAfter += next
			continue
		}
		out.BudgetedAfter += b.Limit.Amount
	}
	for cat, amt := range limits {
		if !replaced[cat] {
			out.BudgetedAfter += amt
		}
	}
	return out
}
