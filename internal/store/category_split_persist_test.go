// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// A receipt imports as one transaction carrying category splits; the splits ride
// along in the existing transactions JSON, so they must survive a store round-trip
// without any schema change.
func TestTransactionSplitsRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	tx := domain.Transaction{
		ID: "t1", AccountID: "a1", Desc: "Costco", Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-1500, "USD"),
		Splits: []domain.CategorySplit{
			{CategoryID: "produce", Amount: money.New(-700, "USD")},
			{CategoryID: "dairy", Amount: money.New(-500, "USD")},
			{CategoryID: "household", Amount: money.New(-300, "USD")},
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
	if len(got.Splits) != 3 {
		t.Fatalf("splits lost in round-trip: %+v", got.Splits)
	}
	if !got.SplitsReconcile() {
		t.Errorf("round-tripped splits should reconcile to the amount: %+v", got)
	}
	if got.Splits[0].CategoryID != "produce" || got.Splits[0].Amount.Amount != -700 {
		t.Errorf("first split wrong after round-trip: %+v", got.Splits[0])
	}
}
