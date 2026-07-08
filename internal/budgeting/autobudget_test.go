// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSuggestBudgets(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-07-15") // July is the current (partial) month → excluded

	cats := []domain.Category{
		{ID: "food", Name: "Food", Kind: domain.KindExpense},
		{ID: "rent", Name: "Rent", Kind: domain.KindExpense},
		{ID: "salary", Name: "Salary", Kind: domain.KindIncome}, // income → never suggested
		{ID: "gifts", Name: "Gifts", Kind: domain.KindExpense},  // no spend → omitted
	}
	txns := []domain.Transaction{
		expense(40000, "USD", "food", "", "2026-06-10"),  // food June
		expense(20000, "USD", "food", "", "2026-05-10"),  // food May (span May..June → /2 = 30000)
		expense(120000, "USD", "rent", "", "2026-06-01"), // rent June only → 120000
	}

	got, err := SuggestBudgets(cats, txns, now, 6, rates, MethodRecent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d suggestions, want 2 (food + rent; income and zero-spend omitted): %+v", len(got), got)
	}
	// Sorted by amount, largest first: rent (120000) before food (30000).
	if got[0].CategoryID != "rent" || got[0].MonthlyMinor != 120000 {
		t.Errorf("first = %+v, want rent/120000", got[0])
	}
	if got[1].CategoryID != "food" || got[1].MonthlyMinor != 30000 {
		t.Errorf("second = %+v, want food/30000", got[1])
	}
}

func TestHealthyLimitDropsSpike(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := mustDate("2026-07-15") // July excluded; span Jan..June (6 full months)

	// Five normal ~$200 months + one $900 blowout (a holiday). Recent mean is inflated;
	// the healthy average drops the spike and reflects a sustainable target.
	txns := []domain.Transaction{
		expense(20000, "USD", "dining", "", "2026-01-10"),
		expense(20000, "USD", "dining", "", "2026-02-10"),
		expense(90000, "USD", "dining", "", "2026-03-10"), // spike
		expense(20000, "USD", "dining", "", "2026-04-10"),
		expense(20000, "USD", "dining", "", "2026-05-10"),
		expense(20000, "USD", "dining", "", "2026-06-10"),
	}
	// Recent mean over the 6-month span: (5×20000 + 90000)/6 = 31666.
	recent, _ := SuggestLimit("dining", txns, now, 6, rates)
	if recent != 31666 {
		t.Errorf("recent mean = %d, want 31666", recent)
	}
	// Healthy: drop the 90000 spike, average the other five: 100000/5 = 20000.
	healthy, _ := HealthyLimit("dining", txns, now, 6, rates)
	if healthy != 20000 {
		t.Errorf("healthy = %d, want 20000 (spike dropped)", healthy)
	}
}
