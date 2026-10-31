// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestAccountDefaultMemberTargetUsesActiveMemberThenOwner(t *testing.T) {
	members := []domain.Member{{ID: "m1", Name: "Alex"}, {ID: "m2", Name: "Sam"}}
	account := domain.Account{OwnerID: "m2"}
	if id, name := accountDefaultMemberTarget(account, members, "m1"); id != "m1" || name != "Alex" {
		t.Fatalf("active member target = %q, %q; want m1, Alex", id, name)
	}
	if id, name := accountDefaultMemberTarget(account, members, ""); id != "m2" || name != "Sam" {
		t.Fatalf("owner target = %q, %q; want m2, Sam", id, name)
	}
	if id, name := accountDefaultMemberTarget(account, members, "missing"); id != "m2" || name != "Sam" {
		t.Fatalf("invalid active member target = %q, %q; want owner m2, Sam", id, name)
	}
}

func TestAccountMenuOffersDefaultActionsAndMarksCurrentDefault(t *testing.T) {
	f := render.New(t)
	calledHousehold, calledMember := false, false
	account := domain.Account{ID: "a1", Name: "Checking", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD"}
	f.Render(ui.CreateElement(AccountRow, accountRowProps{
		Account: account, DefaultMembers: []string{"m1"}, DefaultMemberID: "m2", DefaultMemberName: "Sam",
		OnSetDefaultHousehold: func(id string) { calledHousehold = id == "a1" },
		OnSetDefaultMember:    func(id, memberID string) { calledMember = id == "a1" && memberID == "m2" },
	}))
	if !strings.Contains(f.Text(), "Default") {
		t.Fatal("current default marker is missing")
	}
	openAccountMenu(t, f, "a1")
	findByTestID(f, "button", "acct-default-household-a1").Click()
	f.Stabilize()
	openAccountMenu(t, f, "a1")
	findByTestID(f, "button", "acct-default-member-a1").Click()
	f.Stabilize()
	if !calledHousehold || !calledMember {
		t.Fatalf("default menu callbacks household=%v member=%v", calledHousehold, calledMember)
	}
	householdCleared := false
	f.Render(ui.CreateElement(AccountRow, accountRowProps{
		Account: account, DefaultHousehold: true,
		OnClearDefaultHousehold: func() { householdCleared = true },
	}))
	openAccountMenu(t, f, "a1")
	findByTestID(f, "button", "acct-clear-household-default-a1").Click()
	f.Stabilize()
	if !householdCleared {
		t.Fatal("household default clear callback did not run")
	}
	clearedMember := ""
	f.Render(ui.CreateElement(AccountRow, accountRowProps{
		Account: account, DefaultMembers: []string{"m2"}, DefaultMemberID: "m2", DefaultMemberName: "Sam",
		OnClearDefaultMember: func(memberID string) { clearedMember = memberID },
	}))
	openAccountMenu(t, f, "a1")
	findByTestID(f, "button", "acct-clear-member-default-a1").Click()
	f.Stabilize()
	if clearedMember != "m2" {
		t.Fatalf("cleared member ID = %q, want m2", clearedMember)
	}
}

func TestArchivedAccountCannotOfferDefaultActions(t *testing.T) {
	f := render.New(t)
	account := domain.Account{ID: "a1", Name: "Checking", Type: domain.TypeChecking, Class: domain.ClassAsset, Currency: "USD", Archived: true}
	f.Render(ui.CreateElement(AccountRow, accountRowProps{Account: account, DefaultMemberID: "m1", DefaultMemberName: "Alex"}))
	if strings.Contains(f.Text(), "Make default") {
		t.Fatalf("archived account offers default action: %s", f.Text())
	}
	openAccountMenu(t, f, "a1")
	if findByTestID(f, "button", "acct-default-household-a1") != nil || findByTestID(f, "button", "acct-default-member-a1") != nil {
		t.Fatal("archived account default action is present")
	}
}

// openAccountMenu opens the row's overflow menu, whose entries mount only while it is open.
func openAccountMenu(t *testing.T, f *render.Fixture, accountID string) {
	t.Helper()
	trigger := findByTestID(f, "button", "acct-more-actions-"+accountID)
	if trigger == nil {
		t.Fatalf("account %s has no overflow menu trigger", accountID)
	}
	trigger.Click()
	f.Stabilize()
}
