// SPDX-License-Identifier: MIT

package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPutImportBatchRollsBackRowsWhenHistoryWriteFails(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutDocument(domain.Document{ID: "history", Kind: domain.DocCSV}); err != nil {
		t.Fatal(err)
	}
	txn := domain.Transaction{ID: "new", AccountID: "checking", Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Desc: "Coffee", Amount: money.New(-450, "USD")}
	err = s.PutImportBatch([]domain.Transaction{txn}, domain.Document{ID: "history", Kind: domain.DocCSV})
	if err == nil {
		t.Fatal("expected conflicting history id to fail")
	}
	if _, found, err := s.GetTransaction(txn.ID); err != nil || found {
		t.Fatalf("transaction survived failed import: found=%v err=%v", found, err)
	}
}

func TestPutImportBatchDoesNotOverwriteExistingTransaction(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	existing := domain.Transaction{ID: "same", AccountID: "checking", Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Desc: "Original", Amount: money.New(-450, "USD")}
	if err := s.PutTransaction(existing); err != nil {
		t.Fatal(err)
	}
	replacement := existing
	replacement.Desc = "Replacement"
	if err := s.PutImportBatch([]domain.Transaction{replacement}, domain.Document{ID: "history", Kind: domain.DocCSV}); err == nil {
		t.Fatal("expected transaction ID conflict")
	}
	got, found, err := s.GetTransaction(existing.ID)
	if err != nil || !found || got.Desc != "Original" {
		t.Fatalf("existing transaction after failed import = %+v found=%v err=%v", got, found, err)
	}
}
