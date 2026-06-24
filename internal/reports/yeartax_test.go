// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestYearTaxRollupTotalsAndSort(t *testing.T) {
	start, end := dt(2025, time.January, 1), dt(2026, time.January, 1)
	txns := []domain.Transaction{
		incomeTxn("salary", 4000, dt(2025, time.January, 15)),
		incomeTxn("salary", 4000, dt(2025, time.July, 15)),
		incomeTxn("rental", 2000, dt(2025, time.March, 1)),
		expense("rental", 500, dt(2025, time.April, 1)),
		expense("food", 200, dt(2025, time.June, 12)),
		incomeTxn("salary", 9000, dt(2024, time.December, 31)),                                  // out of range — excluded
		{Amount: money.New(-1000, "USD"), TransferAccountID: "a", Date: dt(2025, time.June, 9)}, // transfer — excluded
	}

	got, err := YearTax(txns, 2025, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year != 2025 {
		t.Errorf("Year = %d, want 2025", got.Year)
	}
	if got.TotalIncome != 1000000 || got.TotalExpense != 70000 || got.NetIncome != 930000 {
		t.Errorf("totals = income %d expense %d net %d; want 1000000 / 70000 / 930000",
			got.TotalIncome, got.TotalExpense, got.NetIncome)
	}
	if len(got.Rows) != 3 {
		t.Fatalf("got %d rows, want 3 (salary, rental, food): %+v", len(got.Rows), got.Rows)
	}
	// Sorted by largest net magnitude: salary (800000) > rental (150000) > food (-20000).
	want := []YearTaxRow{
		{CategoryID: "salary", Income: 800000, Expense: 0, Net: 800000},
		{CategoryID: "rental", Income: 200000, Expense: 50000, Net: 150000},
		{CategoryID: "food", Income: 0, Expense: 20000, Net: -20000},
	}
	for i, w := range want {
		if got.Rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, got.Rows[i], w)
		}
	}
}

func TestYearTaxEmpty(t *testing.T) {
	got, err := YearTax(nil, 2025, dt(2025, time.January, 1), dt(2026, time.January, 1), usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Rows) != 0 || got.TotalIncome != 0 || got.TotalExpense != 0 || got.NetIncome != 0 {
		t.Errorf("empty input should yield a zero summary, got %+v", got)
	}
	if got.Year != 2025 {
		t.Errorf("Year label should pass through even when empty, got %d", got.Year)
	}
}
