// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// incomeTxn builds a non-transfer positive (income) transaction in USD.
func incomeTxn(cat string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{CategoryID: cat, Amount: money.New(major*100, "USD"), Date: on}
}

func TestIncomeByCategorySortedAndExcludes(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		incomeTxn("salary", 4000, dt(2026, time.June, 1)),
		incomeTxn("salary", 4000, dt(2026, time.June, 15)),
		incomeTxn("interest", 50, dt(2026, time.June, 10)),
		incomeTxn("bonus", 1000, dt(2026, time.May, 31)),                                                        // out of range — excluded
		expense("food", 200, dt(2026, time.June, 12)),                                                           // expense — excluded
		{CategoryID: "x", Amount: money.New(9999, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 9)}, // transfer (positive) — excluded
	}
	got, err := IncomeByCategory(txns, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d categories, want 2 (salary, interest): %+v", len(got), got)
	}
	// salary (8000) before interest (50); largest first.
	if got[0].CategoryID != "salary" || got[0].Amount != 800000 {
		t.Errorf("first = %+v, want salary 800000", got[0])
	}
	if got[1].CategoryID != "interest" || got[1].Amount != 5000 {
		t.Errorf("second = %+v, want interest 5000", got[1])
	}
}

func TestLargestIncome(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		incomeTxn("salary", 4000, dt(2026, time.June, 1)),
		incomeTxn("bonus", 1500, dt(2026, time.June, 15)),
		incomeTxn("interest", 50, dt(2026, time.June, 20)),
		incomeTxn("old", 9000, dt(2026, time.May, 31)),                                          // out of range
		expense("food", 200, dt(2026, time.June, 12)),                                           // expense excluded
		{Amount: money.New(-1000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 9)}, // transfer excluded
	}
	got, err := LargestIncome(txns, start, end, usdRates(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want top 2: %+v", len(got), got)
	}
	if got[0].Amount != 400000 || got[1].Amount != 150000 {
		t.Errorf("top two = %d, %d; want 400000, 150000", got[0].Amount, got[1].Amount)
	}
}

func TestIncomeByCategoryEmpty(t *testing.T) {
	got, err := IncomeByCategory(nil, dt(2026, time.June, 1), dt(2026, time.July, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty input should yield no rows, got %+v", got)
	}
}
