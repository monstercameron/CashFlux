// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// payeeExpense builds a non-transfer USD expense with a description.
func payeeExpense(desc string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{Desc: desc, CategoryID: "x", Amount: money.New(-major*100, "USD"), Date: on}
}

func TestTopPayees(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		payeeExpense("Starbucks", 5, dt(2026, time.June, 2)),
		payeeExpense("starbucks", 7, dt(2026, time.June, 9)), // same payee, different case → merges (12)
		payeeExpense("Amazon", 100, dt(2026, time.June, 3)),
		payeeExpense("Amazon", 999, dt(2026, time.May, 30)),                                                    // out of range — excluded
		{Desc: "Paycheck", Amount: money.New(500000, "USD"), Date: dt(2026, time.June, 1)},                     // income — excluded
		{Desc: "Move", Amount: money.New(-20000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 4)}, // transfer — excluded
	}
	got, err := TopPayees(txns, start, end, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d payees, want 2 (Amazon, Starbucks): %+v", len(got), got)
	}
	if got[0].Name != "Amazon" || got[0].Amount != 10000 {
		t.Errorf("row 0 = %+v, want Amazon 10000", got[0])
	}
	// Case-insensitive merge keeps the first spelling and sums to 12.00.
	if got[1].Name != "Starbucks" || got[1].Amount != 1200 {
		t.Errorf("row 1 = %+v, want Starbucks 1200", got[1])
	}
}

func TestTopPayeesLimit(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		payeeExpense("A", 30, dt(2026, time.June, 2)),
		payeeExpense("B", 20, dt(2026, time.June, 2)),
		payeeExpense("C", 10, dt(2026, time.June, 2)),
	}
	got, err := TopPayees(txns, start, end, usdRates(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "A" || got[1].Name != "B" {
		t.Errorf("top-2 = %+v, want A,B", got)
	}
}
