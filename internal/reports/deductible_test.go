package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// cats builds a minimal category list for deductible tests.
// deductible is a set of category ids that should have Deductible=true.
func testCats(deductible ...string) []domain.Category {
	all := []string{"medical", "home-office", "food", "entertainment"}
	dm := make(map[string]bool, len(deductible))
	for _, id := range deductible {
		dm[id] = true
	}
	out := make([]domain.Category, 0, len(all))
	for _, id := range all {
		out = append(out, domain.Category{
			ID:         id,
			Name:       id,
			Kind:       domain.KindExpense,
			Deductible: dm[id],
		})
	}
	return out
}

func TestDeductibleTotals_BasicSums(t *testing.T) {
	start := dt(2025, time.January, 1)
	end := dt(2026, time.January, 1)
	cats := testCats("medical", "home-office")

	txns := []domain.Transaction{
		expense("medical", 300, dt(2025, time.March, 5)),       // deductible
		expense("medical", 200, dt(2025, time.July, 12)),       // deductible
		expense("home-office", 100, dt(2025, time.May, 1)),     // deductible
		expense("food", 400, dt(2025, time.June, 1)),           // NOT deductible
		expense("entertainment", 150, dt(2025, time.April, 3)), // NOT deductible
		// Transfer — must be excluded.
		{Amount: money.New(-999, "USD"), TransferAccountID: "acc-b", Date: dt(2025, time.August, 1)},
		// Out of range — must be excluded.
		expense("medical", 500, dt(2024, time.December, 31)),
	}

	got, err := DeductibleTotals(txns, cats, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 60000 {
		t.Errorf("Total = %d, want 60000 (medical 50000 + home-office 10000)", got.Total)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(got.Rows), got.Rows)
	}
	// Sorted largest first: medical (50000) then home-office (10000).
	if got.Rows[0].CategoryID != "medical" || got.Rows[0].Amount != 50000 {
		t.Errorf("row 0 = %+v, want {medical 50000}", got.Rows[0])
	}
	if got.Rows[1].CategoryID != "home-office" || got.Rows[1].Amount != 10000 {
		t.Errorf("row 1 = %+v, want {home-office 10000}", got.Rows[1])
	}
}

func TestDeductibleTotals_NoDeductibleCategories(t *testing.T) {
	cats := testCats() // none deductible
	txns := []domain.Transaction{
		expense("food", 200, dt(2025, time.May, 1)),
	}
	got, err := DeductibleTotals(txns, cats, dt(2025, time.January, 1), dt(2026, time.January, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 0 || len(got.Rows) != 0 {
		t.Errorf("expected empty summary when no categories are deductible, got %+v", got)
	}
}

func TestDeductibleTotals_EmptyTransactions(t *testing.T) {
	cats := testCats("medical")
	got, err := DeductibleTotals(nil, cats, dt(2025, time.January, 1), dt(2026, time.January, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 0 || len(got.Rows) != 0 {
		t.Errorf("expected empty summary for nil transaction slice, got %+v", got)
	}
}

func TestDeductibleTotals_IncomeExcluded(t *testing.T) {
	cats := testCats("medical")
	txns := []domain.Transaction{
		// Income transaction in a deductible-flagged category — must be excluded.
		incomeTxn("medical", 1000, dt(2025, time.June, 1)),
		expense("medical", 200, dt(2025, time.June, 1)),
	}
	got, err := DeductibleTotals(txns, cats, dt(2025, time.January, 1), dt(2026, time.January, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 20000 {
		t.Errorf("Total = %d, want 20000 (only the expense row)", got.Total)
	}
}

func TestDeductibleCSV_Output(t *testing.T) {
	s := DeductibleSummary{
		Rows: []DeductibleRow{
			{CategoryID: "medical", Amount: 50000},
			{CategoryID: "home-office", Amount: 10000},
		},
		Total: 60000,
	}
	name := func(id string) string { return id }
	amount := func(v int64) string {
		if v == 0 {
			return "0.00"
		}
		return "formatted"
	}
	out := string(DeductibleCSV(s, name, amount))
	if out == "" {
		t.Fatal("DeductibleCSV returned empty bytes")
	}
	// Header row must be present.
	if !contains(out, "Category") || !contains(out, "Deductible Expense") {
		t.Errorf("missing header row in CSV output:\n%s", out)
	}
	// TOTAL row must be present.
	if !contains(out, "TOTAL") {
		t.Errorf("missing TOTAL row in CSV output:\n%s", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
