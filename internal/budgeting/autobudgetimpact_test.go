// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func monthlyBudget(id, cat string, limit int64) domain.Budget {
	return domain.Budget{ID: id, CategoryID: cat, Period: domain.PeriodMonthly, Limit: money.New(limit, "USD")}
}

// A suggestion that REPLACES an existing budget changes the household total by
// the difference, not by its own size. Summing the ticked rows — which is what
// the modal used to show — overstates the effect every time.
func TestAutoBudgetImpactReplacesRatherThanAdds(t *testing.T) {
	budgets := []domain.Budget{monthlyBudget("b1", "groceries", 40000)}
	got := AutoBudgetImpactOf(budgets, 500000, map[string]int64{"groceries": 50000})

	if got.BudgetedBefore != 40000 {
		t.Errorf("BudgetedBefore = %d, want 40000", got.BudgetedBefore)
	}
	if got.BudgetedAfter != 50000 {
		t.Errorf("BudgetedAfter = %d, want 50000 (replaced, not added)", got.BudgetedAfter)
	}
	if got.Change() != 10000 {
		t.Errorf("Change = %d, want 10000 — the difference, not the suggestion's size", got.Change())
	}
	if got.LeftBefore() != 460000 || got.LeftAfter() != 450000 {
		t.Errorf("left = %d → %d, want 460000 → 450000", got.LeftBefore(), got.LeftAfter())
	}
}

func TestAutoBudgetImpactAddsNewCategories(t *testing.T) {
	budgets := []domain.Budget{monthlyBudget("b1", "groceries", 40000)}
	got := AutoBudgetImpactOf(budgets, 0, map[string]int64{"dining": 20000})

	if got.BudgetedAfter != 60000 {
		t.Errorf("BudgetedAfter = %d, want 60000", got.BudgetedAfter)
	}
	if got.HasIncome() {
		t.Error("HasIncome = true with no income basis; the left-over figures would be a lie")
	}
}

// A weekly or yearly limit is not a monthly figure, and folding it into a
// monthly total would produce a number that means nothing.
func TestAutoBudgetImpactIgnoresNonMonthlyBudgets(t *testing.T) {
	budgets := []domain.Budget{
		monthlyBudget("b1", "groceries", 40000),
		{ID: "b2", CategoryID: "coffee", Period: domain.PeriodWeekly, Limit: money.New(5000, "USD")},
	}
	got := AutoBudgetImpactOf(budgets, 0, nil)
	if got.BudgetedBefore != 40000 || got.BudgetedAfter != 40000 {
		t.Errorf("totals = %d/%d, want 40000 both — the weekly budget is not a monthly figure",
			got.BudgetedBefore, got.BudgetedAfter)
	}
}

// Two monthly budgets on one category is a state the duplicate guard prevents,
// but the arithmetic must not double-apply a replacement if it ever occurs.
func TestAutoBudgetImpactReplacesOnlyOnce(t *testing.T) {
	budgets := []domain.Budget{
		monthlyBudget("b1", "groceries", 40000),
		monthlyBudget("b2", "groceries", 10000),
	}
	got := AutoBudgetImpactOf(budgets, 0, map[string]int64{"groceries": 50000})
	if got.BudgetedAfter != 60000 {
		t.Errorf("BudgetedAfter = %d, want 60000 (one replaced, one left alone)", got.BudgetedAfter)
	}
}

func TestAutoBudgetImpactEmpty(t *testing.T) {
	got := AutoBudgetImpactOf(nil, 0, nil)
	if got.BudgetedBefore != 0 || got.BudgetedAfter != 0 || got.Change() != 0 {
		t.Errorf("empty impact = %+v, want zeroes", got)
	}
}
