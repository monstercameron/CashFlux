// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRegularSpendCorrectionOperationsPersistBaselineOnlyChoices(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.PutAccount(domain.Account{ID: "checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutCategory(domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutTransaction(domain.Transaction{
		ID: "large-repair", AccountID: "checking", Desc: "Repair", Amount: money.New(-10000, "USD"),
		Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendEssentialCategory("food", true); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendOneOffTransaction("large-repair", true); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendMerchantRegular("  ACME   Store ", true); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendMerchantRegular("acme store", true); err != nil {
		t.Fatal("repeated correction should be idempotent:", err)
	}
	want := domain.RegularSpendOverrides{
		EssentialCategoryIDs: []string{"food"},
		OneOffTransactionIDs: []string{"large-repair"},
		TreatMerchantRegular: []string{"acme store"},
	}
	if got := a.RegularSpendOverrides(); !reflect.DeepEqual(got, want) {
		t.Fatalf("overrides = %#v, want %#v", got, want)
	}

	if err := a.SetRegularSpendOneOffTransaction("large-repair", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendEssentialCategory("food", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendMerchantRegular("ACME STORE", false); err != nil {
		t.Fatal(err)
	}
	if got := a.RegularSpendOverrides(); len(got.EssentialCategoryIDs)+len(got.OneOffTransactionIDs)+len(got.TreatMerchantRegular) != 0 {
		t.Fatalf("undo left corrections behind: %#v", got)
	}
}

func TestRegularSpendCorrectionOperationsRejectUnknownSources(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetRegularSpendEssentialCategory("missing", true); err == nil {
		t.Error("unknown category was accepted")
	}
	if err := a.SetRegularSpendOneOffTransaction("missing", true); err == nil {
		t.Error("unknown transaction was accepted")
	}
}
