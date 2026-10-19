// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTransactionsSnapshotIsSharedAndInvalidatedByWrite(t *testing.T) {
	a := newApp(t, false)
	first := domain.Transaction{ID: "first", AccountID: "checking"}
	if err := a.store.PutTransaction(first); err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	before := a.Transactions()
	after := a.Transactions()
	if len(before) != 1 || len(after) != 1 {
		t.Fatalf("snapshot lengths = %d and %d, want 1", len(before), len(after))
	}
	if &before[0] != &after[0] {
		t.Fatal("same-revision Transactions calls do not share the backing array")
	}

	second := domain.Transaction{ID: "second", AccountID: "checking"}
	if err := a.store.PutTransaction(second); err != nil {
		t.Fatalf("write transaction: %v", err)
	}

	updated := a.Transactions()
	if len(updated) != 2 {
		t.Fatalf("updated snapshot length = %d, want 2", len(updated))
	}
	if &before[0] == &updated[0] {
		t.Fatal("write did not replace the transaction snapshot backing array")
	}
	if got, ok := a.TransactionByID("second"); !ok || got.ID != second.ID {
		t.Fatalf("TransactionByID(second) = (%q, %v), want second", got.ID, ok)
	}
	if _, ok := a.TransactionByID("missing"); ok {
		t.Fatal("TransactionByID(missing) unexpectedly found a transaction")
	}
}

func TestTransactionsByAccountSnapshotIsShared(t *testing.T) {
	a := newApp(t, false)
	if err := a.store.PutTransactionsAtomic([]domain.Transaction{
		{ID: "one", AccountID: "checking"},
		{ID: "two", AccountID: "savings"},
		{ID: "three", AccountID: "checking"},
	}); err != nil {
		t.Fatalf("seed transactions: %v", err)
	}

	first := a.TransactionsByAccount("checking")
	second := a.TransactionsByAccount("checking")
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("account snapshot lengths = %d and %d, want 2", len(first), len(second))
	}
	if &first[0] != &second[0] {
		t.Fatal("same-revision account calls do not share the backing array")
	}
}

func BenchmarkTransactionsAccessor(b *testing.B) {
	a, err := New(nil, false)
	if err != nil {
		b.Fatalf("New: %v", err)
	}
	const rows = 3300
	txns := make([]domain.Transaction, rows)
	for i := range txns {
		txns[i] = domain.Transaction{ID: fmt.Sprintf("txn-%04d", i), AccountID: "checking"}
	}
	if err := a.store.PutTransactionsAtomic(txns); err != nil {
		b.Fatalf("seed transactions: %v", err)
	}
	a.Transactions()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkTxnRows = a.Transactions()
	}
}

var benchmarkTxnRows []domain.Transaction
