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

// payeeWithFields builds a non-transfer USD expense with explicit Payee and Desc fields.
func payeeWithFields(payee, desc string, major int64, on time.Time) domain.Transaction {
	return domain.Transaction{Payee: payee, Desc: desc, CategoryID: "x", Amount: money.New(-major*100, "USD"), Date: on}
}

func TestTopPayeesTrailing(t *testing.T) {
	asOf := dt(2026, time.July, 1)

	tests := []struct {
		name  string
		txns  []domain.Transaction
		days  int
		limit int
		want  []PayeeSummary
	}{
		{
			name: "basic trailing window with payee fallback to desc",
			txns: []domain.Transaction{
				payeeWithFields("Amazon", "Amazon Prime", 100, dt(2026, time.June, 2)),   // Payee wins
				payeeWithFields("", "Starbucks", 50, dt(2026, time.June, 15)),            // Payee blank → Desc used
				payeeWithFields("Amazon", "Amazon Music", 30, dt(2026, time.March, 1)),   // outside trailing 90 days
			},
			days:  90,
			limit: 10,
			want: []PayeeSummary{
				{Name: "Amazon", Amount: 10000, Count: 1},
				{Name: "Starbucks", Amount: 5000, Count: 1},
			},
		},
		{
			name: "limit respected",
			txns: []domain.Transaction{
				payeeWithFields("A", "", 30, dt(2026, time.June, 2)),
				payeeWithFields("B", "", 20, dt(2026, time.June, 2)),
				payeeWithFields("C", "", 10, dt(2026, time.June, 2)),
			},
			days:  90,
			limit: 2,
			want: []PayeeSummary{
				{Name: "A", Amount: 3000, Count: 1},
				{Name: "B", Amount: 2000, Count: 1},
			},
		},
		{
			name: "count accumulates across transactions for same payee",
			txns: []domain.Transaction{
				payeeWithFields("Cafe", "", 5, dt(2026, time.June, 1)),
				payeeWithFields("Cafe", "", 8, dt(2026, time.June, 10)),
			},
			days:  90,
			limit: 10,
			want: []PayeeSummary{
				{Name: "Cafe", Amount: 1300, Count: 2},
			},
		},
		{
			name: "transfers and income excluded",
			txns: []domain.Transaction{
				{Payee: "Pay", Amount: money.New(500000, "USD"), Date: dt(2026, time.June, 1)},                        // income (positive) — excluded
				{Payee: "Move", Amount: money.New(-20000, "USD"), TransferAccountID: "a", Date: dt(2026, time.June, 4)}, // transfer — excluded
				payeeWithFields("Expense", "", 10, dt(2026, time.June, 5)),
			},
			days:  90,
			limit: 10,
			want: []PayeeSummary{
				{Name: "Expense", Amount: 1000, Count: 1},
			},
		},
		{
			name: "blank name (no payee, no desc) skipped",
			txns: []domain.Transaction{
				{Amount: money.New(-1000, "USD"), CategoryID: "x", Date: dt(2026, time.June, 1)}, // no Payee, no Desc
				payeeWithFields("Real", "", 5, dt(2026, time.June, 2)),
			},
			days:  90,
			limit: 10,
			want: []PayeeSummary{
				{Name: "Real", Amount: 500, Count: 1},
			},
		},
		{
			name: "tie broken alphabetically",
			txns: []domain.Transaction{
				payeeWithFields("Zebra", "", 10, dt(2026, time.June, 1)),
				payeeWithFields("Alpha", "", 10, dt(2026, time.June, 1)),
			},
			days:  90,
			limit: 10,
			want: []PayeeSummary{
				{Name: "Alpha", Amount: 1000, Count: 1},
				{Name: "Zebra", Amount: 1000, Count: 1},
			},
		},
		{
			name: "no qualifying transactions returns empty slice",
			txns: []domain.Transaction{
				payeeWithFields("Old", "", 10, dt(2026, time.January, 1)), // outside window
			},
			days:  90,
			limit: 10,
			want:  []PayeeSummary{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TopPayeesTrailing(tc.txns, tc.days, asOf, usdRates(), tc.limit)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d payees, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("row %d: got %+v, want %+v", i, got[i], w)
				}
			}
		})
	}
}
