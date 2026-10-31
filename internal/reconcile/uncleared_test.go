// SPDX-License-Identifier: MIT

package reconcile

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestUnclearedKeepsOnlyThisAccountsUnclearedRows(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "a1", AccountID: "a", Cleared: false},
		{ID: "a2", AccountID: "a", Cleared: true},
		{ID: "b1", AccountID: "b", Cleared: false},
		{ID: "a3", AccountID: "a", Cleared: false},
	}
	tests := []struct {
		name      string
		accountID string
		txns      []domain.Transaction
		want      []string
	}{
		{"uncleared rows of the account in order", "a", txns, []string{"a1", "a3"}},
		{"another account", "b", txns, []string{"b1"}},
		{"unknown account", "z", txns, nil},
		{"no transactions", "a", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Uncleared(tt.accountID, tt.txns)
			if len(got) != len(tt.want) {
				t.Fatalf("Uncleared = %d rows, want %d", len(got), len(tt.want))
			}
			for i, id := range tt.want {
				if got[i].ID != id {
					t.Fatalf("row %d = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}
