// SPDX-License-Identifier: MIT

package store

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestRegularSpendOverridesSQLiteAndDatasetRoundTrip(t *testing.T) {
	s := newStore(t)
	want := domain.RegularSpendOverrides{
		EssentialCategoryIDs: []string{" food ", "food", "home"},
		OneOffTransactionIDs: []string{"txn-2", "txn-1"},
		TreatMerchantRegular: []string{"  ACME   CO ", "acme co", "Shop"},
	}
	if err := s.PutRegularSpendOverrides(want); err != nil {
		t.Fatalf("PutRegularSpendOverrides: %v", err)
	}
	want = domain.RegularSpendOverrides{
		EssentialCategoryIDs: []string{"food", "home"},
		OneOffTransactionIDs: []string{"txn-1", "txn-2"},
		TreatMerchantRegular: []string{"acme co", "shop"},
	}
	got, err := s.GetRegularSpendOverrides()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("GetRegularSpendOverrides = %#v, %v; want %#v", got, err, want)
	}

	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	exported, err := Export(snapshot)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	imported, err := Import(exported)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	copy := newStore(t)
	if err := copy.Load(imported); err != nil {
		t.Fatalf("Load imported dataset: %v", err)
	}
	loaded, err := copy.GetRegularSpendOverrides()
	if err != nil || !reflect.DeepEqual(loaded, want) {
		t.Fatalf("round-trip overrides = %#v, %v; want %#v", loaded, err, want)
	}
}

func TestRegularSpendOverridesAreHouseholdStoreScoped(t *testing.T) {
	first, second := newStore(t), newStore(t)
	if err := first.PutRegularSpendOverrides(domain.RegularSpendOverrides{EssentialCategoryIDs: []string{"first"}}); err != nil {
		t.Fatal(err)
	}
	got, err := second.GetRegularSpendOverrides()
	if err != nil || len(got.EssentialCategoryIDs) != 0 {
		t.Fatalf("other household inherited overrides: %#v, %v", got, err)
	}
}

func TestDeleteCategoryAndTransactionCleanOverridesAtomically(t *testing.T) {
	s := newStore(t)
	if err := s.PutCategory(domain.Category{ID: "food", Name: "Food"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutTransaction(domain.Transaction{ID: "one-off"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutRegularSpendOverrides(domain.RegularSpendOverrides{
		EssentialCategoryIDs: []string{"food", "home"},
		OneOffTransactionIDs: []string{"one-off", "keep"},
	}); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.DeleteCategoryWithRegularSpendCleanup("food")
	if err != nil || !deleted {
		t.Fatalf("DeleteCategoryWithRegularSpendCleanup = %v, %v", deleted, err)
	}
	deleted, err = s.DeleteTransactionWithRegularSpendCleanup("one-off")
	if err != nil || !deleted {
		t.Fatalf("DeleteTransactionWithRegularSpendCleanup = %v, %v", deleted, err)
	}
	got, err := s.GetRegularSpendOverrides()
	if err != nil {
		t.Fatal(err)
	}
	want := domain.RegularSpendOverrides{EssentialCategoryIDs: []string{"home"}, OneOffTransactionIDs: []string{"keep"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("overrides after delete = %#v, want %#v", got, want)
	}
}
