// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestQuickAddAccountResolverDF4(t *testing.T) {
	accounts := []domain.Account{
		{ID: "checking-a", Type: domain.TypeChecking, Class: domain.ClassAsset},
		{ID: "checking-b", Type: domain.TypeChecking, Class: domain.ClassAsset},
		{ID: "archived", Type: domain.TypeChecking, Class: domain.ClassAsset, Archived: true},
	}
	members := []domain.Member{{ID: "member", Prefs: domain.MemberPrefs{DefaultAccountID: "checking-b"}}}
	input := accountselect.ResolveInput{
		Accounts: accounts, ActiveMemberID: "member", Members: members,
		WorkspaceDefaultID: "checking-a",
	}
	tests := []struct {
		name  string
		input accountselect.ResolveInput
		want  string
	}{
		{name: "member default", input: input, want: "checking-b"},
		{name: "workspace fallback", input: accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "checking-a"}, want: "checking-a"},
		{name: "archived member default skipped", input: accountselect.ResolveInput{Accounts: accounts, ActiveMemberID: "member", Members: []domain.Member{{ID: "member", Prefs: domain.MemberPrefs{DefaultAccountID: "archived"}}}, WorkspaceDefaultID: "checking-a"}, want: "checking-a"},
		{name: "missing member default skipped", input: accountselect.ResolveInput{Accounts: accounts, ActiveMemberID: "member", Members: []domain.Member{{ID: "member", Prefs: domain.MemberPrefs{DefaultAccountID: "missing"}}}, WorkspaceDefaultID: "checking-a"}, want: "checking-a"},
		{name: "template account is explicit", input: accountselect.ResolveInput{Accounts: accounts, ExplicitID: "checking-a", ActiveMemberID: "member", Members: members, WorkspaceDefaultID: "checking-a"}, want: "checking-a"},
		{name: "transfer source stays explicit", input: accountselect.ResolveInput{Accounts: accounts, ExplicitID: "checking-a", WorkspaceDefaultID: "checking-b"}, want: "checking-a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveQuickAddAccountID(tc.input); got != tc.want {
				t.Fatalf("resolveQuickAddAccountID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestQuickAddSaveAndAnotherResolvesAccountAgainDF4(t *testing.T) {
	appState, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "checking-a", Name: "Checking A", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "checking-b", Name: "Checking B", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := appState.PutAccount(account); err != nil {
			t.Fatalf("PutAccount(%s): %v", account.ID, err)
		}
	}
	settings := appState.Settings()
	settings.DefaultAccountID = "checking-b"
	if err := appState.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	previous := appstate.Default
	appstate.Default = appState
	t.Cleanup(func() { appstate.Default = previous; uistate.SetQuickAdd(false) })

	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("df4-body"), ui.CreateElement(QuickAddHost)))
	uistate.SetQuickAdd(true)
	f.Render(Body(ID("df4-body"), ui.CreateElement(QuickAddHost)))
	if !quickAddAccountSelected(f, "Checking B · Checking") {
		t.Fatal("initial account selection is not Checking B")
	}
	findAppTestID(f, "input", "txn-add-amount").Input("12.50")
	findAppTestID(f, "input", "txn-add-payee").Input("Lunch")
	findAppTestID(f, "button", "txn-add-another").Click()
	f.Stabilize()
	if got := appState.Transactions(); len(got) != 1 || got[0].AccountID != "checking-b" {
		t.Fatalf("transactions after Save & add another = %#v, want one on checking-b", got)
	}
	if !quickAddAccountSelected(f, "Checking B · Checking") {
		t.Fatal("account after Save & add another is not Checking B")
	}
}

func quickAddAccountSelected(f *render.Fixture, label string) bool {
	for _, option := range f.AllByTag("option") {
		if option.Text() == label && option.Property("selected") == true {
			return true
		}
	}
	return false
}
