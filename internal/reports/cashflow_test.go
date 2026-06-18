package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// income builds a non-transfer positive transaction in USD.
func income(major int64, on time.Time) domain.Transaction {
	return domain.Transaction{Amount: money.New(major*100, "USD"), Date: on}
}

func TestIncomeVsExpense(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		income(4000, dt(2026, time.June, 1)),
		expense("food", 1000, dt(2026, time.June, 5)),
		expense("rent", 1000, dt(2026, time.June, 6)),
		income(9999, dt(2026, time.May, 30)),                                                    // out of range
		{Amount: money.New(-5000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 7)}, // transfer excluded
	}
	f, err := IncomeVsExpense(txns, start, end, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Income != 400000 || f.Expense != 200000 {
		t.Fatalf("income/expense = %d/%d, want 400000/200000", f.Income, f.Expense)
	}
	if f.Net() != 200000 {
		t.Errorf("Net = %d, want 200000", f.Net())
	}
	if f.SavingsRate() != 50 {
		t.Errorf("SavingsRate = %d, want 50", f.SavingsRate())
	}
}

func TestIncomeExpenseSeries(t *testing.T) {
	bounds := []time.Time{
		dt(2026, time.April, 1),
		dt(2026, time.May, 1),
		dt(2026, time.June, 1),
	}
	txns := []domain.Transaction{
		income(1000, dt(2026, time.April, 10)),
		expense("x", 400, dt(2026, time.April, 12)),
		expense("x", 700, dt(2026, time.May, 15)),
	}
	got, err := IncomeExpenseSeries(txns, bounds, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2", len(got))
	}
	// April: income 1000, expense 400.
	if got[0].Income != 100000 || got[0].Expense != 40000 {
		t.Errorf("bucket 0 = %d/%d, want 100000/40000", got[0].Income, got[0].Expense)
	}
	// May: income 0, expense 700 (net negative).
	if got[1].Income != 0 || got[1].Expense != 70000 || got[1].Net() != -70000 {
		t.Errorf("bucket 1 = %d/%d net %d, want 0/70000 net -70000", got[1].Income, got[1].Expense, got[1].Net())
	}
}

func TestIncomeExpenseSeriesTooFewBounds(t *testing.T) {
	got, err := IncomeExpenseSeries(nil, []time.Time{dt(2026, time.June, 1)}, usdRates())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d, want 0 for a single bound", len(got))
	}
}
