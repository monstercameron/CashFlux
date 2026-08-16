// SPDX-License-Identifier: MIT

package budgeting

import (
	"strconv"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
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

// Both suggestion methods must read the same household. A family that always
// splits its receipt across categories got a Groceries proposal under "recent"
// and none at all under "healthy", because HealthyLimit counted only whole
// transactions booked to the category (the split contract's category-side rule).
func TestHealthyLimitAttributesSplitLines(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	split := func(day int, groceries, household int64) domain.Transaction {
		return domain.Transaction{
			ID: "r" + strconv.Itoa(day), CategoryID: "shopping",
			Amount: money.New(-(groceries + household), "USD"),
			Date:   time.Date(2026, time.June, day, 0, 0, 0, 0, time.UTC),
			Splits: []domain.CategorySplit{
				{CategoryID: "groceries", Amount: money.New(-groceries, "USD")},
				{CategoryID: "household", Amount: money.New(-household, "USD")},
			},
		}
	}
	txns := []domain.Transaction{split(5, 8000, 4000), split(20, 6000, 2000)}

	got, err := HealthyLimit("groceries", txns, now, 6, rates)
	if err != nil {
		t.Fatalf("HealthyLimit: %v", err)
	}
	if got != 14000 {
		t.Errorf("HealthyLimit = %d, want 14000 (both June grocery lines)", got)
	}

	recent, err := SuggestLimit("groceries", txns, now, 6, rates)
	if err != nil {
		t.Fatalf("SuggestLimit: %v", err)
	}
	if recent != got {
		t.Errorf("the two methods disagree on a one-month span: recent=%d healthy=%d", recent, got)
	}
}
