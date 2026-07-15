// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestAccountGroupCRUD(t *testing.T) {
	a := newApp(t, false)

	// Name is required.
	if _, err := a.PutAccountGroup(domain.AccountGroup{AccountIDs: []string{"x"}}); err == nil {
		t.Fatal("expected error for a nameless group")
	}

	// Create two groups; Order defaults to the end of the list.
	g1, err := a.PutAccountGroup(domain.AccountGroup{Name: "Liquid", AccountIDs: []string{"chk", "sav"}})
	if err != nil {
		t.Fatalf("PutAccountGroup g1: %v", err)
	}
	if g1.ID == "" || g1.Order != 1 {
		t.Fatalf("g1 = %+v, want id + order 1", g1)
	}
	g2, err := a.PutAccountGroup(domain.AccountGroup{Name: "Property", AccountIDs: []string{"house"}})
	if err != nil {
		t.Fatalf("PutAccountGroup g2: %v", err)
	}
	if g2.Order != 2 {
		t.Fatalf("g2 order = %d, want 2", g2.Order)
	}

	// Single-membership: assigning "sav" into g2 removes it from g1.
	g2.AccountIDs = []string{"house", "sav"}
	if _, err := a.PutAccountGroup(g2); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	groups := a.AccountGroups()
	byID := map[string]domain.AccountGroup{}
	for _, g := range groups {
		byID[g.ID] = g
	}
	if byID[g1.ID].HasAccount("sav") {
		t.Errorf("g1 should no longer contain sav: %+v", byID[g1.ID])
	}
	if !byID[g2.ID].HasAccount("sav") {
		t.Errorf("g2 should contain sav: %+v", byID[g2.ID])
	}

	// Delete only ungroups (accounts untouched); the group disappears.
	if err := a.DeleteAccountGroup(g1.ID); err != nil {
		t.Fatalf("DeleteAccountGroup: %v", err)
	}
	if _, ok := a.GetAccountGroup(g1.ID); ok {
		t.Error("g1 should be gone after delete")
	}
	if len(a.AccountGroups()) != 1 {
		t.Errorf("want 1 group left, got %d", len(a.AccountGroups()))
	}
}
