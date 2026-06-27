// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// basePropertyAccount returns a minimal valid property account for snapshot tests.
func basePropertyAccount(id string, balanceMinor int64, asOf time.Time) domain.Account {
	return domain.Account{
		ID:             id,
		Name:           "My House",
		Currency:       "USD",
		Type:           domain.TypeProperty,
		Class:          domain.ClassAsset,
		OwnerID:        domain.GroupOwnerID,
		Scope:          domain.ScopeShared,
		OpeningBalance: money.Money{Amount: balanceMinor, Currency: "USD"},
		BalanceAsOf:    asOf,
	}
}

// TestPutAccountRecordsSnapshotOnBalanceChange verifies that PutAccount writes a
// BalanceSnapshot when the OpeningBalance differs from the prior persisted value.
// A new account with a nonzero balance counts as a "change" from the implicit zero.
// BalanceHistory returns snapshots in ascending chronological order.
func TestPutAccountRecordsSnapshotOnBalanceChange(t *testing.T) {
	a := newApp(t, false)

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	acct := basePropertyAccount("prop1", 30000000, t0) // $300,000

	// First Put: new account with nonzero balance → records initial snapshot.
	if err := a.PutAccount(acct); err != nil {
		t.Fatalf("PutAccount (initial): %v", err)
	}
	h0 := a.BalanceHistory("prop1")
	if len(h0) != 1 {
		t.Fatalf("initial put: expected 1 snapshot (initial balance), got %d", len(h0))
	}
	if h0[0].BalanceMinor != 30000000 {
		t.Errorf("initial snapshot balance = %d, want 30000000", h0[0].BalanceMinor)
	}

	// Second Put: balance increases → total snapshots = 2.
	acct.OpeningBalance = money.Money{Amount: 31000000, Currency: "USD"}
	acct.BalanceAsOf = t1
	if err := a.PutAccount(acct); err != nil {
		t.Fatalf("PutAccount (balance increase): %v", err)
	}
	h1 := a.BalanceHistory("prop1")
	if len(h1) != 2 {
		t.Fatalf("after first balance change: expected 2 snapshots, got %d", len(h1))
	}
	if h1[1].BalanceMinor != 31000000 {
		t.Errorf("second snapshot balance = %d, want 31000000", h1[1].BalanceMinor)
	}
	if !h1[1].AsOf.Equal(t1) {
		t.Errorf("second snapshot AsOf = %v, want %v", h1[1].AsOf, t1)
	}
	if h1[1].AccountID != "prop1" {
		t.Errorf("snapshot AccountID = %q, want prop1", h1[1].AccountID)
	}
	if h1[1].Currency != "USD" {
		t.Errorf("snapshot Currency = %q, want USD", h1[1].Currency)
	}

	// Snapshots must be in ascending time order.
	if !h1[0].AsOf.Before(h1[1].AsOf) {
		t.Errorf("BalanceHistory not ascending: [0].AsOf=%v [1].AsOf=%v", h1[0].AsOf, h1[1].AsOf)
	}

	// Third Put: balance changes again → total = 3, still ascending.
	acct.OpeningBalance = money.Money{Amount: 32000000, Currency: "USD"}
	acct.BalanceAsOf = t2
	if err := a.PutAccount(acct); err != nil {
		t.Fatalf("PutAccount (second balance change): %v", err)
	}
	h2 := a.BalanceHistory("prop1")
	if len(h2) != 3 {
		t.Fatalf("after second balance change: expected 3 snapshots, got %d", len(h2))
	}
	for i := 1; i < len(h2); i++ {
		if !h2[i-1].AsOf.Before(h2[i].AsOf) {
			t.Errorf("BalanceHistory not ascending at index %d: %v vs %v", i, h2[i-1].AsOf, h2[i].AsOf)
		}
	}
}

// TestPutAccountNoSnapshotWhenBalanceUnchanged verifies that re-saving an account
// with the same balance (e.g., a name rename) does NOT record a new snapshot.
func TestPutAccountNoSnapshotWhenBalanceUnchanged(t *testing.T) {
	a := newApp(t, false)

	asOf := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	acct := basePropertyAccount("prop2", 50000000, asOf)

	// Initial save → 1 snapshot (initial balance).
	if err := a.PutAccount(acct); err != nil {
		t.Fatalf("PutAccount (initial): %v", err)
	}
	if h := a.BalanceHistory("prop2"); len(h) != 1 {
		t.Fatalf("initial put: expected 1 snapshot, got %d", len(h))
	}

	// Change only the name — balance unchanged → still 1 snapshot.
	acct.Name = "My Rental Property"
	if err := a.PutAccount(acct); err != nil {
		t.Fatalf("PutAccount (name change): %v", err)
	}
	if h := a.BalanceHistory("prop2"); len(h) != 1 {
		t.Errorf("name-only change: expected still 1 snapshot, got %d", len(h))
	}
}

// TestBalanceHistoryIsolatedPerAccount verifies snapshots for one account are not
// returned when querying a different account.
func TestBalanceHistoryIsolatedPerAccount(t *testing.T) {
	a := newApp(t, false)

	asOf := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	acctA := basePropertyAccount("propA", 40000000, asOf)
	if err := a.PutAccount(acctA); err != nil {
		t.Fatalf("PutAccount A initial: %v", err)
	}
	acctA.OpeningBalance = money.Money{Amount: 41000000, Currency: "USD"}
	acctA.BalanceAsOf = asOf.Add(24 * time.Hour)
	if err := a.PutAccount(acctA); err != nil {
		t.Fatalf("PutAccount A updated: %v", err)
	}

	// Account A: 2 snapshots (initial + update).
	// Account B: 1 snapshot (initial nonzero balance).
	acctB := basePropertyAccount("propB", 20000000, asOf)
	acctB.Name = "Second Property"
	if err := a.PutAccount(acctB); err != nil {
		t.Fatalf("PutAccount B: %v", err)
	}

	if h := a.BalanceHistory("propA"); len(h) != 2 {
		t.Errorf("propA: expected 2 snapshots, got %d", len(h))
	}
	if h := a.BalanceHistory("propB"); len(h) != 1 {
		t.Errorf("propB: expected 1 snapshot, got %d", len(h))
	}

	// propA snapshots must not appear in propB's history.
	for _, s := range a.BalanceHistory("propB") {
		if s.AccountID != "propB" {
			t.Errorf("propB history contains snapshot for wrong account: %q", s.AccountID)
		}
	}
}
