// SPDX-License-Identifier: MIT

package payees_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payees"
)

func txn(payee, desc string, dayOffset int) domain.Transaction {
	return domain.Transaction{
		Payee: payee,
		Desc:  desc,
		Date:  time.Date(2024, 1, 1+dayOffset, 0, 0, 0, 0, time.UTC),
	}
}

func TestRecentPayees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		txns  []domain.Transaction
		limit int
		want  []string
	}{
		{
			name:  "empty input",
			txns:  nil,
			limit: 10,
			want:  []string{},
		},
		{
			name:  "blank payees and descs are skipped",
			txns:  []domain.Transaction{txn("", "", 0), txn("  ", "  ", 1)},
			limit: 10,
			want:  []string{},
		},
		{
			name:  "payee preferred over desc",
			txns:  []domain.Transaction{txn("Whole Foods", "WHOLEFDS #42", 0)},
			limit: 10,
			want:  []string{"Whole Foods"},
		},
		{
			name:  "desc used when payee blank",
			txns:  []domain.Transaction{txn("", "Netflix", 0)},
			limit: 10,
			want:  []string{"Netflix"},
		},
		{
			name: "dedup case-insensitive, first casing (newest) preserved",
			txns: []domain.Transaction{
				txn("Amazon", "", 5), // most recent
				txn("amazon", "", 3),
				txn("AMAZON", "", 1),
			},
			limit: 10,
			want:  []string{"Amazon"},
		},
		{
			name: "ordered newest-first",
			txns: []domain.Transaction{
				txn("Old Place", "", 0),
				txn("Middle Spot", "", 5),
				txn("New Shop", "", 10),
			},
			limit: 10,
			want:  []string{"New Shop", "Middle Spot", "Old Place"},
		},
		{
			name: "limit respected",
			txns: []domain.Transaction{
				txn("A", "", 3),
				txn("B", "", 2),
				txn("C", "", 1),
			},
			limit: 2,
			want:  []string{"A", "B"},
		},
		{
			name: "limit zero returns all",
			txns: []domain.Transaction{
				txn("A", "", 3),
				txn("B", "", 2),
			},
			limit: 0,
			want:  []string{"A", "B"},
		},
		{
			name:  "whitespace trimmed",
			txns:  []domain.Transaction{txn("  Starbucks  ", "", 0)},
			limit: 10,
			want:  []string{"Starbucks"},
		},
		{
			name: "mixed payee and desc fallback",
			txns: []domain.Transaction{
				txn("Whole Foods", "", 5),
				txn("", "Netflix", 3),
				txn("Whole Foods", "WF desc", 1), // dup — should be skipped
			},
			limit: 10,
			want:  []string{"Whole Foods", "Netflix"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := payees.RecentPayees(tc.txns, tc.limit)
			if len(got) != len(tc.want) {
				t.Fatalf("len=%d want=%d; got=%v want=%v", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] got=%q want=%q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
