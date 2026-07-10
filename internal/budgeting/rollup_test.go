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

// TestEvaluateRollupMultiCategory reproduces the "Mortgage Payment · HOA" budget:
// a multi-category budget must sum spend across ALL its tracked categories. It also
// pins the date boundary that explains the reported "not adding up": a payment made
// on the LAST day of the PRIOR month is out of this month's range, so it doesn't count.
func TestEvaluateRollupMultiCategory(t *testing.T) {
	start, end := june() // [2026-06-01, 2026-07-01)
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{
		CategoryIDs: []string{"hoa", "mortgage"}, // multi-category budget
		Scope:       domain.ScopeShared,
		Limit:       usd(170000), // $1,700
	}
	covers := map[string]bool{"hoa": true, "mortgage": true}

	// Both bills IN the period sum: $1,302.10 + $376.86 = $1,678.96.
	both := []domain.Transaction{
		expense(130210, "USD", "mortgage", "", "2026-06-05"),
		expense(37686, "USD", "hoa", "", "2026-06-15"),
	}
	if st, _ := EvaluateRollup(budget, both, start, end, rates, DefaultNearThreshold, covers); !st.Spent.Equal(usd(167896)) {
		t.Errorf("both in-period: spent = %v, want 167896 ($1,678.96 = mortgage + HOA)", st.Spent)
	}

	// The HOA payment on the LAST day of the PRIOR month (May 31) is out of the June
	// range, so only the mortgage counts — the user's actual situation.
	crossMonth := []domain.Transaction{
		expense(130210, "USD", "mortgage", "", "2026-06-05"),
		expense(37686, "USD", "hoa", "", "2026-05-31"),
	}
	if st, _ := EvaluateRollup(budget, crossMonth, start, end, rates, DefaultNearThreshold, covers); !st.Spent.Equal(usd(130210)) {
		t.Errorf("cross-month: spent = %v, want 130210 (mortgage only; May 31 HOA is prior month)", st.Spent)
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
