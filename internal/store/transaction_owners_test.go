// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTransactionOwnersSQLiteAndDatasetRoundTrip(t *testing.T) {
	s := newStore(t)
	if err := s.PutTransaction(domain.Transaction{ID: "shared", MemberIDs: []string{"a", "b", "a"}}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetTransaction("shared")
	if err != nil || !ok {
		t.Fatalf("GetTransaction: ok=%v err=%v", ok, err)
	}
	if got.MemberID != "" || strings.Join(got.Owners(), ",") != "a,b" {
		t.Fatalf("SQLite owners = %#v", got)
	}
	byMember, err := s.TransactionsByMember("b")
	if err != nil || len(byMember) != 1 || byMember[0].ID != "shared" {
		t.Fatalf("TransactionsByMember = %#v, err=%v", byMember, err)
	}
	blob, err := Export(Dataset{Transactions: []domain.Transaction{got}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), `"memberId":`) || !strings.Contains(string(blob), `"memberIds"`) {
		t.Fatalf("noncanonical export: %s", blob)
	}
	imported, err := Import(blob)
	if err != nil || len(imported.Transactions) != 1 || strings.Join(imported.Transactions[0].Owners(), ",") != "a,b" {
		t.Fatalf("dataset round-trip: %#v err=%v", imported.Transactions, err)
	}
}

func TestLegacySQLiteTransactionMigratesOnRead(t *testing.T) {
	s := newStore(t)
	if _, err := s.db.Exec(`INSERT INTO transactions(id, data) VALUES('legacy', '{"id":"legacy","memberId":"old"}')`); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetTransaction("legacy")
	if err != nil || !ok || got.MemberID != "" || !got.HasOwner("old") {
		t.Fatalf("legacy SQLite row = %#v, ok=%v, err=%v", got, ok, err)
	}
}

func TestLegacyTransactionDatasetMigratesOnLoad(t *testing.T) {
	var dataset Dataset
	if err := json.Unmarshal([]byte(`{"schemaVersion":1,"transactions":[{"id":"old","memberId":"m1"}]}`), &dataset); err != nil {
		t.Fatal(err)
	}
	if len(dataset.Transactions) != 1 || dataset.Transactions[0].MemberID != "" || strings.Join(dataset.Transactions[0].Owners(), ",") != "m1" {
		t.Fatalf("legacy transaction did not migrate: %#v", dataset.Transactions)
	}
}
