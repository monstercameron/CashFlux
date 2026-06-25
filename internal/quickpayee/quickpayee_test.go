// SPDX-License-Identifier: MIT

package quickpayee_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/quickpayee"
)

// base date for tests.
var base = time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

// txn builds a Transaction with the given payee, desc, and date offset (days from base).
func txn(payee, desc string, daysFromBase int) domain.Transaction {
	return domain.Transaction{
		ID:    fmt.Sprintf("t-%s-%d", payee+desc, daysFromBase),
		Date:  base.AddDate(0, 0, daysFromBase),
		Payee: payee,
		Desc:  desc,
	}
}

func TestRecentPayees(t *testing.T) {
	tests := []struct {
		name string
		txns []domain.Transaction
		n    int
		want []string
	}{
		{
			name: "empty input returns nil",
			txns: nil,
			n:    0,
			want: nil,
		},
		{
			name: "single transaction with payee",
			txns: []domain.Transaction{txn("Walmart", "", 0)},
			n:    0,
			want: []string{"Walmart"},
		},
		{
			name: "Desc fallback when Payee is empty",
			txns: []domain.Transaction{txn("", "Rent payment", 0)},
			n:    0,
			want: []string{"Rent payment"},
		},
		{
			name: "both Payee and Desc empty — skip row",
			txns: []domain.Transaction{txn("", "", 0)},
			n:    0,
			want: nil,
		},
		{
			name: "Payee takes priority over Desc",
			txns: []domain.Transaction{txn("Costco", "ignored desc", 0)},
			n:    0,
			want: []string{"Costco"},
		},
		{
			name: "dedup is case-insensitive, first casing preserved",
			txns: []domain.Transaction{
				txn("Starbucks", "", 2), // more recent → first
				txn("starbucks", "", 1), // dup (lower)
				txn("STARBUCKS", "", 0), // dup (upper)
			},
			n:    0,
			want: []string{"Starbucks"},
		},
		{
			name: "result is ordered most-recent-first",
			txns: []domain.Transaction{
				txn("Target", "", 0),   // oldest
				txn("Amazon", "", 2),   // newest
				txn("Walmart", "", 1),  // middle
			},
			n:    0,
			want: []string{"Amazon", "Walmart", "Target"},
		},
		{
			name: "n positive limits scan to n most-recent transactions",
			txns: []domain.Transaction{
				txn("Amazon", "", 3),  // most recent
				txn("Walmart", "", 2),
				txn("Target", "", 1),
				txn("Costco", "", 0), // oldest — outside n=3 window
			},
			n:    3,
			want: []string{"Amazon", "Walmart", "Target"},
		},
		{
			name: "n zero scans all",
			txns: []domain.Transaction{
				txn("A", "", 3),
				txn("B", "", 2),
				txn("C", "", 1),
				txn("D", "", 0),
			},
			n:    0,
			want: []string{"A", "B", "C", "D"},
		},
		{
			name: "n negative scans all (treated as no limit)",
			txns: []domain.Transaction{
				txn("A", "", 1),
				txn("B", "", 0),
			},
			n:    -5,
			want: []string{"A", "B"},
		},
		{
			name: "result is capped at 20",
			txns: func() []domain.Transaction {
				out := make([]domain.Transaction, 25)
				for i := 0; i < 25; i++ {
					out[i] = txn(fmt.Sprintf("Payee%02d", i), "", i)
				}
				return out
			}(),
			n:    0,
			want: func() []string {
				// sorted descending: Payee24 → Payee05 (first 20 distinct)
				out := make([]string, 20)
				for i := 0; i < 20; i++ {
					out[i] = fmt.Sprintf("Payee%02d", 24-i)
				}
				return out
			}(),
		},
		{
			name: "Desc fallback: mixed payee/no-payee rows",
			txns: []domain.Transaction{
				txn("", "Utility bill", 3),  // Desc fallback, most recent
				txn("Netflix", "", 2),
				txn("", "Gym fee", 1),        // Desc fallback
				txn("Netflix", "", 0),        // dup of Netflix
			},
			n:    0,
			want: []string{"Utility bill", "Netflix", "Gym fee"},
		},
		{
			name: "n larger than slice length scans all",
			txns: []domain.Transaction{
				txn("A", "", 1),
				txn("B", "", 0),
			},
			n:    1000,
			want: []string{"A", "B"},
		},
		{
			name: "identical dates — stable sort preserves input order within same day",
			txns: []domain.Transaction{
				// same date, different payees — sort is stable so original order holds
				{ID: "t1", Date: base, Payee: "First"},
				{ID: "t2", Date: base, Payee: "Second"},
			},
			n:    0,
			want: []string{"First", "Second"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quickpayee.RecentPayees(tc.txns, tc.n)

			if len(got) != len(tc.want) {
				t.Fatalf("RecentPayees() len=%d, want len=%d\ngot:  %v\nwant: %v",
					len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("RecentPayees()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}

			// Verify the cap is never exceeded regardless of input.
			if len(got) > 20 {
				t.Errorf("RecentPayees() returned %d entries, must be ≤ 20", len(got))
			}
		})
	}
}
