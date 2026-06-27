// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// C122: OnTxnMutated observer fires on PutTransaction (new add).
func TestOnTxnMutated_FiresOnAdd(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)

	var fired int
	a.OnTxnMutated(func() { fired++ })

	tx := domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Desc: "Coffee", Amount: money.New(-500, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	if fired != 1 {
		t.Errorf("observer should fire once on add, fired=%d", fired)
	}
}

// C122: OnTxnMutated observer fires on PutTransaction (edit of existing txn).
func TestOnTxnMutated_FiresOnEdit(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)

	tx := domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Desc: "Coffee", Amount: money.New(-500, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("initial add: %v", err)
	}

	var fired int
	a.OnTxnMutated(func() { fired++ })

	tx.Desc = "Coffee (edited)"
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if fired != 1 {
		t.Errorf("observer should fire on edit too, fired=%d", fired)
	}
}

// C122: OnTxnMutated observer fires on DeleteTransaction.
func TestOnTxnMutated_FiresOnDelete(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)

	tx := domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Desc: "Coffee", Amount: money.New(-500, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("add: %v", err)
	}

	var fired int
	a.OnTxnMutated(func() { fired++ })

	if err := a.DeleteTransaction("t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if fired != 1 {
		t.Errorf("observer should fire on delete, fired=%d", fired)
	}
}

// C122 import-storm guard: bulk CSV import fires the observer exactly once,
// regardless of how many rows are imported, not once per row.
func TestOnTxnMutated_ImportFiresOnce(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)

	var fired int
	a.OnTxnMutated(func() { fired++ })

	csv := "date,account_id,desc,amount\n2026-06-10,Checking,Row one,-10\n2026-06-11,Checking,Row two,-20\n2026-06-12,Checking,Row three,-30\n"
	n, _, err := a.ImportTransactionsCSV([]byte(csv), "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 3 {
		t.Fatalf("imported %d rows, want 3", n)
	}
	if fired != 1 {
		t.Errorf("import-storm guard: observer should fire exactly once for a batch, fired=%d", fired)
	}
}

// C122 import-storm guard: re-importing the same CSV (all rows are dupes, n=0)
// must NOT fire the observer at all (nothing changed).
func TestOnTxnMutated_ImportNoFireOnNoop(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)

	csv := "date,account_id,desc,amount\n2026-06-10,Checking,Coffee,-10\n"
	if n, _, err := a.ImportTransactionsCSV([]byte(csv), ""); err != nil || n != 1 {
		t.Fatalf("first import: n=%d err=%v", n, err)
	}

	var fired int
	a.OnTxnMutated(func() { fired++ })

	// Re-import the same file: all rows are duplicates, n=0 → no fire.
	n, _, err := a.ImportTransactionsCSV([]byte(csv), "")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new rows, got %d", n)
	}
	if fired != 0 {
		t.Errorf("no-op import must not fire observer, fired=%d", fired)
	}
}
