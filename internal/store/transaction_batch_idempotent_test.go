// SPDX-License-Identifier: MIT

package store

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestPutTransferBatchAtomicFailureAtEachInsert(t *testing.T) {
	for failAt := 1; failAt <= 3; failAt++ {
		t.Run(fmt.Sprintf("insert_%d", failAt), func(t *testing.T) {
			s, err := NewMemory()
			if err != nil {
				t.Fatalf("NewMemory: %v", err)
			}
			defer s.Close()
			txns := []domain.Transaction{{ID: "leg-out"}, {ID: "leg-in"}, {ID: "fee"}}
			target := txns[failAt-1].ID
			if _, err := s.db.Exec(fmt.Sprintf(`CREATE TRIGGER fail_transfer_insert BEFORE INSERT ON transactions
				WHEN NEW.id = '%s' BEGIN SELECT RAISE(FAIL, 'injected transfer insert failure'); END`, target)); err != nil {
				t.Fatalf("create failure trigger: %v", err)
			}
			if _, err := s.PutTransferBatchAtomic("operation", txns); err == nil {
				t.Fatal("PutTransferBatchAtomic succeeded despite injected insert failure")
			}
			got, err := s.ListTransactions()
			if err != nil {
				t.Fatalf("ListTransactions: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("partial rows after failed batch = %d, want 0", len(got))
			}
		})
	}
}

func TestPutTransferBatchAtomicRetryAndConflict(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer s.Close()

	batch := []domain.Transaction{{ID: "leg-out", Payee: "destination"}, {ID: "leg-in", Payee: "source"}}
	if _, err := s.PutTransferBatchAtomic("operation", batch); err != nil {
		t.Fatalf("initial batch: %v", err)
	}
	if _, err := s.PutTransferBatchAtomic("operation", batch); err != nil {
		t.Fatalf("identical retry: %v", err)
	}
	conflict := []domain.Transaction{{ID: "leg-out", Payee: "different operation"}, {ID: "leg-in", Payee: "source"}}
	if _, err := s.PutTransferBatchAtomic("operation", conflict); err == nil {
		t.Fatal("conflicting operation payload accepted")
	}
	got, err := s.ListTransactions()
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("row count after retry/conflict = %d, want 2", len(got))
	}
}

func TestPutTransferBatchAtomicRetryAfterReceiptRebuild(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer s.Close()
	batch := []domain.Transaction{{ID: "leg-out", Payee: "destination"}, {ID: "leg-in", Payee: "source"}}
	if _, err := s.PutTransferBatchAtomic("operation", batch); err != nil {
		t.Fatalf("initial batch: %v", err)
	}
	// The auxiliary receipt table is not part of exported transaction data and
	// may be rebuilt on reload; stable leg IDs and exact row comparison preserve
	// idempotency across that boundary.
	if _, err := s.db.Exec("DELETE FROM transfer_operations"); err != nil {
		t.Fatalf("remove receipt: %v", err)
	}
	inserted, err := s.PutTransferBatchAtomic("operation", batch)
	if err != nil {
		t.Fatalf("retry after receipt rebuild: %v", err)
	}
	if inserted {
		t.Fatal("identical rows after receipt rebuild reported a new write")
	}
}
