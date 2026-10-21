// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
)

func TestRevCacheCheckedRetriesFailedLoadAndReturnsOwnedSlice(t *testing.T) {
	var cache revCache[int]
	attempts := 0
	load := func() ([]int, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("temporary read failure")
		}
		return []int{7, 11}, nil
	}

	if got, err := cache.getChecked(42, load); err == nil || got != nil {
		t.Fatalf("first checked read = %v, %v; want a failed read", got, err)
	}
	if cache.loaded {
		t.Fatal("failed read marked cache loaded")
	}
	first, err := cache.getChecked(42, load)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || len(first) != 2 || first[0] != 7 || first[1] != 11 {
		t.Fatalf("retry result = %v after %d attempts", first, attempts)
	}
	first[0] = 99
	second, err := cache.getChecked(42, func() ([]int, error) {
		t.Fatal("warm read unexpectedly called loader")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0] != 7 || second[1] != 11 {
		t.Fatalf("caller mutation changed cached value: %v", second)
	}
}

func TestCategorySourcesCheckedPreserveStorageFailures(t *testing.T) {
	a := newApp(t, false)
	if err := a.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TransactionsChecked(); err == nil {
		t.Fatal("TransactionsChecked hid the closed-store error")
	}
	if _, err := a.TxnLinksChecked(); err == nil {
		t.Fatal("TxnLinksChecked hid the closed-store error")
	}
	if _, err := a.CategoriesChecked(); err == nil {
		t.Fatal("CategoriesChecked hid the closed-store error")
	}
	if _, err := a.AccountsChecked(); err == nil {
		t.Fatal("AccountsChecked hid the closed-store error")
	}
	if _, err := a.GoalsChecked(); err == nil {
		t.Fatal("GoalsChecked hid the closed-store error")
	}
	if _, err := a.BudgetsChecked(); err == nil {
		t.Fatal("BudgetsChecked hid the closed-store error")
	}
}

func TestCheckedAccountAndGoalSnapshotsRejectClosedWarmStore(t *testing.T) {
	a := newApp(t, false)
	if _, err := a.AccountsChecked(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GoalsChecked(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.BudgetsChecked(); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AccountsChecked(); err == nil {
		t.Fatal("warm AccountsChecked cache hid a closed store")
	}
	if _, err := a.GoalsChecked(); err == nil {
		t.Fatal("warm GoalsChecked cache hid a closed store")
	}
	if _, err := a.BudgetsChecked(); err == nil {
		t.Fatal("warm BudgetsChecked cache hid a closed store")
	}
}
