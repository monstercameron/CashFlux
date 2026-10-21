// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestListAllBalanceSnapshotsIncludesOrphanHistoryAndReadFailure(t *testing.T) {
	s := newStore(t)
	for _, row := range []domain.BalanceSnapshot{{ID: "b", AccountID: "removed-account", BalanceMinor: 42, Currency: "EUR"}, {ID: "a", AccountID: "other-account", BalanceMinor: 99, Currency: "USD"}} {
		if err := s.PutBalanceSnapshot(row); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.ListAllBalanceSnapshots()
	if err != nil || len(rows) != 2 || rows[0].ID != "a" || rows[1].ID != "b" || rows[1].AccountID != "removed-account" {
		t.Fatalf("lost retained history: %+v %v", rows, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListAllBalanceSnapshots(); err == nil {
		t.Fatal("closed store became healthy empty history")
	}
}
