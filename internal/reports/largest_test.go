package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestLargestExpenses(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		payeeExpense("Rent", 900, dt(2026, time.June, 1)),
		payeeExpense("Groceries", 150, dt(2026, time.June, 10)),
		payeeExpense("Laptop", 1200, dt(2026, time.June, 15)),
		payeeExpense("Old", 9999, dt(2026, time.May, 20)),                                                      // out of range
		{Desc: "Pay", Amount: money.New(500000, "USD"), Date: dt(2026, time.June, 2)},                          // income
		{Desc: "Move", Amount: money.New(-70000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 3)}, // transfer
	}
	got, err := LargestExpenses(txns, start, end, usdRates(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(got), got)
	}
	// Largest first: Laptop 1200, Rent 900, Groceries 150.
	if got[0].Desc != "Laptop" || got[0].Amount != 120000 {
		t.Errorf("row 0 = %+v, want Laptop 120000", got[0])
	}
	if got[1].Desc != "Rent" || got[2].Desc != "Groceries" {
		t.Errorf("order = %s,%s,%s, want Laptop,Rent,Groceries", got[0].Desc, got[1].Desc, got[2].Desc)
	}
}

func TestLargestExpensesLimitAndTieBreak(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		payeeExpense("A", 100, dt(2026, time.June, 5)), // tie on amount; newer date wins
		payeeExpense("B", 100, dt(2026, time.June, 10)),
		payeeExpense("C", 50, dt(2026, time.June, 1)),
	}
	got, err := LargestExpenses(txns, start, end, usdRates(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want top 2", len(got))
	}
	// Both A and B are 100; B is more recent so it sorts first.
	if got[0].Desc != "B" || got[1].Desc != "A" {
		t.Errorf("tie order = %s,%s, want B,A (newer first)", got[0].Desc, got[1].Desc)
	}
}
