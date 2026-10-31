// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// putGroupTestAccounts stores one shared checking account per id.
func putGroupTestAccounts(t *testing.T, a *App, ids ...string) {
	t.Helper()
	for _, accountID := range ids {
		acc := domain.Account{ID: accountID, Name: accountID, Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
		if err := a.PutAccount(acc); err != nil {
			t.Fatalf("put account %s: %v", accountID, err)
		}
	}
}

func TestAccountGroupCRUD(t *testing.T) {
	a := newApp(t, false)
	putGroupTestAccounts(t, a, "chk", "sav", "house")

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

func TestAccountGroupWritesRefuseAViewer(t *testing.T) {
	a := newApp(t, false)
	putGroupTestAccounts(t, a, "chk", "sav")
	g1, err := a.PutAccountGroup(domain.AccountGroup{Name: "Liquid", AccountIDs: []string{"chk"}})
	if err != nil {
		t.Fatal(err)
	}
	g2, err := a.PutAccountGroup(domain.AccountGroup{Name: "Other", AccountIDs: []string{"sav"}})
	if err != nil {
		t.Fatal(err)
	}
	a.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	g1.AccountIDs = []string{"chk", "sav"}
	if _, err := a.PutAccountGroup(g1); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("PutAccountGroup as viewer: err = %v, want ErrReadOnly", err)
	}
	if err := a.DeleteAccountGroup(g2.ID); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("DeleteAccountGroup as viewer: err = %v, want ErrReadOnly", err)
	}
	got1, _ := a.GetAccountGroup(g1.ID)
	got2, ok := a.GetAccountGroup(g2.ID)
	if len(got1.AccountIDs) != 1 || !ok || len(got2.AccountIDs) != 1 {
		t.Fatalf("a refused write changed groups: %+v %+v", got1, got2)
	}
}

func TestPutAccountGroupRefusesUnknownAccount(t *testing.T) {
	a := newApp(t, false)
	putGroupTestAccounts(t, a, "chk")
	_, err := a.PutAccountGroup(domain.AccountGroup{Name: "Liquid", AccountIDs: []string{"chk", "ghost"}})
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
	if n := len(a.AccountGroups()); n != 0 {
		t.Fatalf("%d groups saved after a refused write", n)
	}
}

func TestPutAccountGroupMoveIsOneCommit(t *testing.T) {
	a := newApp(t, false)
	putGroupTestAccounts(t, a, "chk", "sav", "house")
	src, err := a.PutAccountGroup(domain.AccountGroup{Name: "Liquid", AccountIDs: []string{"chk", "sav"}})
	if err != nil {
		t.Fatal(err)
	}
	other, err := a.PutAccountGroup(domain.AccountGroup{Name: "Property", AccountIDs: []string{"house"}})
	if err != nil {
		t.Fatal(err)
	}

	// A move that takes accounts from two groups lands as one commit: the store revision
	// moves once, so a failure part-way cannot leave one group written and the other not.
	rev := a.store.Rev()
	dest, err := a.PutAccountGroup(domain.AccountGroup{Name: "Everything", AccountIDs: []string{"sav", "house"}})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if got := a.store.Rev() - rev; got != 1 {
		t.Fatalf("a move touching three groups took %d commits, want 1", got)
	}
	for groupID, want := range map[string][]string{src.ID: {"chk"}, other.ID: {}, dest.ID: {"sav", "house"}} {
		got, ok := a.GetAccountGroup(groupID)
		if !ok || len(got.AccountIDs) != len(want) {
			t.Fatalf("group %s members = %v, want %v", groupID, got.AccountIDs, want)
		}
	}
}
