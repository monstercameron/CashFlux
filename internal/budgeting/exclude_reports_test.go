// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestSpentExcludesExcludedFromReports: a transaction marked ExcludeFromReports
// does not count toward budget spend (TXC-1).
func TestSpentExcludesExcludedFromReports(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	d := func(day int) time.Time { return time.Date(2026, time.June, day, 0, 0, 0, 0, time.UTC) }
	start, end := d(1), time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	budget := domain.Budget{ID: "b", CategoryID: "food", Limit: money.New(50000, "USD"), OwnerID: domain.GroupOwnerID}
	txns := []domain.Transaction{
		{ID: "n", CategoryID: "food", Amount: money.New(-2000, "USD"), Date: d(5)},
		{ID: "x", CategoryID: "food", Amount: money.New(-9000, "USD"), Date: d(6), ExcludeFromReports: true},
	}
	spent, err := Spent(budget, txns, start, end, rates)
	if err != nil {
		t.Fatal(err)
	}
	if spent.Amount != 2000 {
		t.Errorf("Spent = %d, want 2000 (the $90 excluded reimbursement must not count against the budget)", spent.Amount)
	}
}
