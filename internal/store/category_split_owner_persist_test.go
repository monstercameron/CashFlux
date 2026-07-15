// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestTransactionSplitOwnerRoundTrip verifies XC10: a split line's owner
// (MemberID) rides along in the transactions JSON and survives an export/import
// round-trip, so owner-aware budget attribution stays correct after a reload.
func TestTransactionSplitOwnerRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	tx := domain.Transaction{
		ID: "t1", AccountID: "a1", Desc: "Costco", MemberID: "A",
		Date:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-1000, "USD"),
		Splits: []domain.CategorySplit{
			{CategoryID: "groceries", Amount: money.New(-600, "USD")},            // no owner → payer A
			{CategoryID: "hobby", Amount: money.New(-400, "USD"), MemberID: "B"}, // owned by B
		},
	}
	if err := st.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	ds, err := st.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	imported, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(imported.Transactions) != 1 {
		t.Fatalf("want 1 transaction, got %d", len(imported.Transactions))
	}
	got := imported.Transactions[0]
	if len(got.Splits) != 2 {
		t.Fatalf("splits lost in round-trip: %+v", got.Splits)
	}
	if got.Splits[0].MemberID != "" {
		t.Errorf("unowned line should stay empty (fall back to payer), got %q", got.Splits[0].MemberID)
	}
	if got.Splits[1].MemberID != "B" {
		t.Errorf("owned line lost its owner: got %q, want B", got.Splits[1].MemberID)
	}
	// The effective owner resolves correctly after the round-trip.
	if o := got.Splits[0].LineOwner(got.MemberID); o != "A" {
		t.Errorf("unowned line owner = %q, want payer A", o)
	}
	if o := got.Splits[1].LineOwner(got.MemberID); o != "B" {
		t.Errorf("owned line owner = %q, want B", o)
	}
}
