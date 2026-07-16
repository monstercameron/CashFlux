// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestReportsExcludeFromReports: excluded transactions drop out of the reporting
// aggregations (category spend, largest expenses) (TXC-1).
func TestReportsExcludeFromReports(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	d := func(day int) time.Time { return time.Date(2026, time.June, day, 0, 0, 0, 0, time.UTC) }
	start, end := d(1), time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "n", CategoryID: "food", Desc: "Groceries", Amount: money.New(-2000, "USD"), Date: d(5)},
		{ID: "x", CategoryID: "food", Desc: "Reimbursed lunch", Amount: money.New(-9000, "USD"), Date: d(6), ExcludeFromReports: true},
	}

	rows, err := SpendingByCategory(txns, start, end, false, time.Time{}, time.Time{}, rates)
	if err != nil {
		t.Fatal(err)
	}
	var food int64
	for _, r := range rows {
		if r.CategoryID == "food" {
			food = r.Amount
		}
	}
	if food != 2000 {
		t.Errorf("SpendingByCategory[food] = %d, want 2000 (excluded txn must not count)", food)
	}

	big, err := LargestExpenses(txns, start, end, rates, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range big {
		if e.Desc == "Reimbursed lunch" {
			t.Errorf("LargestExpenses included an excluded transaction: %+v", e)
		}
	}
}
