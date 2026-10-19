// SPDX-License-Identifier: MIT

package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTV8GMigratedTransferGroupsRoundTrip(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	input := Dataset{SchemaVersion: SchemaVersion,
		Accounts: []domain.Account{
			{ID: "checking", Class: domain.ClassAsset, Currency: "USD"},
			{ID: "mortgage", Class: domain.ClassLiability, Type: domain.TypeMortgage, Currency: "USD", LiabilitySign: domain.LiabilityPositiveOwed},
		},
		Transactions: []domain.Transaction{
			{ID: "cash", AccountID: "checking", TransferAccountID: "mortgage", Date: day, Amount: money.New(-148000, "USD")},
			{ID: "debt", AccountID: "mortgage", TransferAccountID: "checking", Date: day, Amount: money.New(-148000, "USD")},
		},
	}
	migrated := backfillLegacyTransferGroups(input)
	if input.Transactions[0].TransferGroupID != "" || migrated.Transactions[0].TransferGroupID == "" || migrated.Transactions[0].TransferGroupID != migrated.Transactions[1].TransferGroupID {
		t.Fatal("migration did not copy and link the two mortgage legs")
	}
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Load(migrated); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Export(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Import(encoded)
	if err != nil {
		t.Fatal(err)
	}
	again := backfillLegacyTransferGroups(decoded)
	if !reflect.DeepEqual(again.Transactions, snapshot.Transactions) {
		t.Fatal("load, export, import and repeated migration changed transfer identities")
	}
}

func TestTV8GSampleMortgageAndOwnersDrawAreBackfilled(t *testing.T) {
	ds := SampleDatasetAt(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
	migrated := backfillLegacyTransferGroups(ds)
	counts := map[string]int{}
	for _, row := range migrated.Transactions {
		if row.Desc != "Mortgage payment" && !strings.HasPrefix(row.Desc, "Owner's draw") {
			continue
		}
		counts[row.Desc]++
		if row.TransferGroupID == "" {
			t.Errorf("sample transfer %s (%s) has no durable group", row.ID, row.Desc)
		}
	}
	if len(counts) != 2 {
		t.Fatalf("sample fixture did not cover both reported transfers: %v", counts)
	}
}
