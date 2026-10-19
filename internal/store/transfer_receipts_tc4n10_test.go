// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTC4N10DeletingTransferPrunesReceiptAndExportPayload(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Load(EmptyDataset()); err != nil {
		t.Fatal(err)
	}

	txns := []domain.Transaction{
		{ID: "transfer-out", Desc: "private merchant details", Amount: money.New(-4200, "USD")},
		{ID: "transfer-in", Desc: "private merchant details", Amount: money.New(4200, "USD")},
	}
	if changed, err := s.PutTransferBatchAtomic("operation", txns); err != nil || !changed {
		t.Fatalf("create transfer: changed=%v err=%v", changed, err)
	}
	if _, err := s.DeleteTransactionsAtomic([]string{"transfer-out", "transfer-in"}); err != nil {
		t.Fatal(err)
	}
	var receiptCount int
	if err := s.db.QueryRow("SELECT count(*) FROM transfer_operations").Scan(&receiptCount); err != nil {
		t.Fatal(err)
	}
	if receiptCount != 0 {
		t.Fatalf("deleted transfer retained %d operation receipts", receiptCount)
	}

	ds, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.TransferOperations) != 0 {
		t.Fatalf("snapshot retained deleted transfer receipt: %+v", ds.TransferOperations)
	}
	data, err := Export(ds)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private merchant details") {
		t.Fatal("export retained deleted transfer details")
	}
	if changed, err := s.PutTransferBatchAtomic("operation", txns); err != nil || !changed {
		t.Fatalf("explicitly deleted operation could not be retried: changed=%v err=%v", changed, err)
	}
}

func TestTC4N10ExportAndImportDropReceiptsForMissingRows(t *testing.T) {
	receipt := TransferOperationReceipt{
		ID:           "stale-operation",
		Transactions: []domain.Transaction{{ID: "deleted-row", Desc: "must not escape"}},
	}
	ds := Dataset{TransferOperations: []TransferOperationReceipt{receipt}}
	data, err := Export(ds)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "must not escape") {
		t.Fatal("export serialized a receipt for a missing transaction")
	}
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, present := envelope["transferOperations"]; present {
		t.Fatalf("empty receipt field should be omitted: %s", data)
	}

	legacy := `{"schemaVersion":1,"transferOperations":[{"id":"stale-operation","transactions":[{"id":"deleted-row","description":"must not escape"}]}]}`
	imported, err := Import([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.TransferOperations) != 0 {
		t.Fatalf("import retained receipt for missing transaction: %+v", imported.TransferOperations)
	}
}

func TestTC4N10SnapshotPrunesLegacyDeletedReceipt(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Load(EmptyDataset()); err != nil {
		t.Fatal(err)
	}
	if err := s.PutTransaction(domain.Transaction{ID: "old-transfer-leg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteTransaction("old-transfer-leg"); err != nil {
		t.Fatal(err)
	}
	// Simulate a receipt written by a pre-fix build after its transaction was deleted.
	if _, err := s.db.Exec(`INSERT INTO transfer_operations(id, data) VALUES(?, ?)`, "legacy-operation", `[{"id":"old-transfer-leg","description":"legacy secret"}]`); err != nil {
		t.Fatal(err)
	}
	ds, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.TransferOperations) != 0 {
		t.Fatalf("snapshot retained legacy stale receipt: %+v", ds.TransferOperations)
	}
	var receiptCount int
	if err := s.db.QueryRow("SELECT count(*) FROM transfer_operations").Scan(&receiptCount); err != nil {
		t.Fatal(err)
	}
	if receiptCount != 0 {
		t.Fatalf("snapshot failed to prune legacy receipt; count=%d", receiptCount)
	}
}
