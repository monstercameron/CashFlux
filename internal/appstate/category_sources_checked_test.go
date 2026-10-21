// SPDX-License-Identifier: MIT

package appstate

import "testing"

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
