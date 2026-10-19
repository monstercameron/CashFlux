// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func sampleDebtPaymentRecords() ([]domain.DebtPaymentLink, []domain.DebtPayeeRule) {
	links := []domain.DebtPaymentLink{{
		ID: "link-1", TransactionIDs: []string{"txn-1", "txn-2"}, DebtAccountID: "loan",
		Currency: "USD", PrincipalMinor: 8200, InterestMinor: 1200, FeesMinor: 300,
		Method: domain.DebtPaymentStatement, Source: domain.DebtPaymentConfirmed, CreatedRevision: 42,
	}}
	rules := []domain.DebtPayeeRule{{
		ID: "rule-1", WorkspaceID: "workspace-1", Payee: "LENDER PAYMENT", DebtAccountID: "loan",
		SourceAccountID: "checking", CreatedAt: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
	}}
	return links, rules
}

func TestDebtPaymentDatasetJSONRoundTripAndLegacyCompatibility(t *testing.T) {
	links, rules := sampleDebtPaymentRecords()
	ds := Dataset{DebtPaymentLinks: links, DebtPayeeRules: rules}
	encoded, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	decoded, err := Import(encoded)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	reencoded, err := Export(decoded)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("debt-payment dataset JSON changed across round-trip\nfirst: %s\nsecond: %s", encoded, reencoded)
	}

	legacy, err := Import([]byte(`{"schemaVersion":1,"members":[],"accounts":[],"categories":[],"transactions":[],"budgets":[],"goals":[],"tasks":[],"settings":{}}`))
	if err != nil {
		t.Fatalf("legacy Import: %v", err)
	}
	if len(legacy.DebtPaymentLinks) != 0 || len(legacy.DebtPayeeRules) != 0 {
		t.Fatalf("legacy dataset debt records = (%v, %v), want empty", legacy.DebtPaymentLinks, legacy.DebtPayeeRules)
	}
	var emptyJSON map[string]json.RawMessage
	empty, err := Export(Dataset{})
	if err != nil {
		t.Fatalf("Export empty dataset: %v", err)
	}
	if err := json.Unmarshal(empty, &emptyJSON); err != nil {
		t.Fatal(err)
	}
	if _, ok := emptyJSON["debtPaymentLinks"]; ok {
		t.Fatal("empty debtPaymentLinks field was not omitted")
	}
	if _, ok := emptyJSON["debtPayeeRules"]; ok {
		t.Fatal("empty debtPayeeRules field was not omitted")
	}
}

func TestSQLiteDatasetLoadSnapshotRoundTripsDebtPayments(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	links, rules := sampleDebtPaymentRecords()
	want := Dataset{DebtPaymentLinks: links, DebtPayeeRules: rules}
	if err := s.Load(want); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(got.DebtPaymentLinks) != 1 || got.DebtPaymentLinks[0].ID != links[0].ID || len(got.DebtPaymentLinks[0].TransactionIDs) != 2 {
		t.Fatalf("Snapshot debt links = %#v", got.DebtPaymentLinks)
	}
	if len(got.DebtPayeeRules) != 1 || got.DebtPayeeRules[0] != rules[0] {
		t.Fatalf("Snapshot debt rules = %#v", got.DebtPayeeRules)
	}

	if err := s.Load(Dataset{}); err != nil {
		t.Fatalf("Load empty dataset: %v", err)
	}
	got, err = s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after empty Load: %v", err)
	}
	if len(got.DebtPaymentLinks) != 0 || len(got.DebtPayeeRules) != 0 {
		t.Fatalf("empty Load left debt-payment data behind: %#v / %#v", got.DebtPaymentLinks, got.DebtPayeeRules)
	}
}
