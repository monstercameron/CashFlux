// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPutTransactionsAtomicRollsBackEarlierRows(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, txn := range []domain.Transaction{{ID: "a", Desc: "old a"}, {ID: "b", Desc: "old b"}} {
		if err := s.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_second_update BEFORE UPDATE ON transactions
		WHEN OLD.id = 'b' BEGIN SELECT RAISE(ABORT, 'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	err = s.PutTransactionsAtomic([]domain.Transaction{{ID: "a", Desc: "new a", Amount: money.New(1, "USD")},
		{ID: "b", Desc: "new b", Amount: money.New(2, "USD")}})
	if err == nil {
		t.Fatal("PutTransactionsAtomic succeeded despite the second-row failure")
	}
	for id, want := range map[string]string{"a": "old a", "b": "old b"} {
		got, ok, err := s.GetTransaction(id)
		if err != nil || !ok || got.Desc != want {
			t.Errorf("transaction %s after rollback = (%+v, %t, %v), want desc %q", id, got, ok, err, want)
		}
	}
}

func TestDeleteTransactionsAtomicRollsBackEarlierRows(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, txn := range []domain.Transaction{{ID: "a"}, {ID: "b"}} {
		if err := s.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_second_delete BEFORE DELETE ON transactions
		WHEN OLD.id = 'b' BEGIN SELECT RAISE(ABORT, 'injected delete failure'); END`); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.DeleteTransactionsAtomic([]string{"a", "b"})
	if err == nil {
		t.Fatal("DeleteTransactionsAtomic succeeded despite the second-row failure")
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted IDs on failed transaction = %v, want none", deleted)
	}
	for _, id := range []string{"a", "b"} {
		if _, ok, err := s.GetTransaction(id); err != nil || !ok {
			t.Errorf("transaction %s missing after rollback (ok=%t, err=%v)", id, ok, err)
		}
	}
}
