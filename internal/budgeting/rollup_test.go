// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestEvaluateRollupIncludesDescendants(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(10000)} // $100
	all := []domain.Transaction{
		expense(2000, "USD", "food", "", "2026-06-03"), // $20 on the parent
		expense(3000, "USD", "groc", "", "2026-06-04"), // $30 on a sub-category
		expense(1000, "USD", "rent", "", "2026-06-05"), // unrelated, excluded
	}
	// Without rollup, only the budget's own category counts ($20).
	if st, _ := Evaluate(budget, all, start, end, rates, DefaultNearThreshold); !st.Spent.Equal(usd(2000)) {
		t.Errorf("Evaluate spent = %v, want 2000 (food only)", st.Spent)
	}
	// With rollup, the descendant category counts too ($20 + $30 = $50).
	covers := map[string]bool{"food": true, "groc": true}
	st, err := EvaluateRollup(budget, all, start, end, rates, DefaultNearThreshold, covers)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !st.Spent.Equal(usd(5000)) {
		t.Errorf("EvaluateRollup spent = %v, want 5000 (food + groc)", st.Spent)
	}
	if st.Percent != 50 {
		t.Errorf("percent = %d, want 50", st.Percent)
	}
}

func TestEvaluateRollupEmptyCoversIsOwnCategory(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(10000)}
	all := []domain.Transaction{
		expense(2000, "USD", "food", "", "2026-06-03"),
		expense(3000, "USD", "groc", "", "2026-06-04"),
	}
	st, _ := EvaluateRollup(budget, all, start, end, rates, DefaultNearThreshold, nil)
	if !st.Spent.Equal(usd(2000)) {
		t.Errorf("nil covers spent = %v, want 2000 (own category only)", st.Spent)
	}
}

func TestEvaluateRollupRespectsScope(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: usd(10000)}
	all := []domain.Transaction{
		expense(2000, "USD", "food", "m1", "2026-06-03"), // owner parent, counts
		expense(3000, "USD", "groc", "m1", "2026-06-04"), // owner sub-category, counts via rollup
		expense(4000, "USD", "groc", "m2", "2026-06-05"), // other member's sub-category, excluded by scope
	}
	covers := map[string]bool{"food": true, "groc": true}
	st, _ := EvaluateRollup(budget, all, start, end, rates, DefaultNearThreshold, covers)
	if !st.Spent.Equal(usd(5000)) {
		t.Errorf("scoped rollup spent = %v, want 5000 (m1's food + groc, not m2)", st.Spent)
	}
}
