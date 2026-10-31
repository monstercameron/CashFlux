// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestListAllTransferReceiptsRetainsLegacyRowsWithoutPruningOrSchemaWrites(t *testing.T) {
	s := newStore(t)
	if _, err := s.db.Exec("DROP TRIGGER IF EXISTS prune_transfer_receipts_after_transaction_delete"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"z-operation", "a-operation"} {
		legs := []domain.Transaction{{ID: "missing-leg", Desc: "Historical receipt", Amount: money.New(math.MinInt64, "JPY")}}
		b, _ := json.Marshal(legs)
		if _, err := s.db.Exec("INSERT INTO transfer_operations(id,data) VALUES(?,?)", id, string(b)); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.ListAllTransferReceipts()
	if err != nil || len(rows) != 2 || rows[0].ID != "a-operation" || rows[1].ID != "z-operation" || rows[0].Transactions[0].Amount.Amount != math.MinInt64 || rows[0].Transactions[0].Amount.Currency != "JPY" {
		t.Fatal("lost exact retained receipt/ordering", rows, err)
	}
	var retained, triggers int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM transfer_operations").Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name='prune_transfer_receipts_after_transaction_delete'").Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if retained != 2 || triggers != 0 {
		t.Fatal("read pruned or installed cleanup trigger", retained, triggers)
	}
}

func TestListAllTransferReceiptsPropagatesCorruptionAndClosedStore(t *testing.T) {
	s := newStore(t)
	if _, err := s.db.Exec("INSERT INTO transfer_operations(id,data) VALUES(?,?)", "broken", "{bad"); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ListAllTransferReceipts(); err == nil || len(rows) != 0 {
		t.Fatal("corruption became healthy partial/empty receipts", rows, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListAllTransferReceipts(); err == nil {
		t.Fatal("closed source became healthy empty receipts")
	}
}
