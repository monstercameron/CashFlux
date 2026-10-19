// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestInitialImportAccountResolverDF4(t *testing.T) {
	accounts := []domain.Account{
		{ID: "checking-a", Type: domain.TypeChecking, Class: domain.ClassAsset},
		{ID: "checking-b", Type: domain.TypeChecking, Class: domain.ClassAsset},
		{ID: "archived", Type: domain.TypeChecking, Class: domain.ClassAsset, Archived: true},
	}
	tests := []struct {
		name  string
		input accountselect.ResolveInput
		want  string
	}{
		{
			name: "member default",
			input: accountselect.ResolveInput{Accounts: accounts, ActiveMemberID: "member", Members: []domain.Member{
				{ID: "member", Prefs: domain.MemberPrefs{DefaultAccountID: "checking-b"}},
			}, WorkspaceDefaultID: "checking-a"},
			want: "checking-b",
		},
		{name: "household fallback", input: accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "checking-a"}, want: "checking-a"},
		{
			name:  "archived saved default skipped",
			input: accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "archived"},
			want:  "checking-a",
		},
		{
			name:  "missing saved default skipped",
			input: accountselect.ResolveInput{Accounts: accounts, WorkspaceDefaultID: "missing"},
			want:  "checking-a",
		},
		{name: "no accounts", input: accountselect.ResolveInput{}, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveInitialImportAccount(tc.input); got != tc.want {
				t.Fatalf("resolveInitialImportAccount() = %q, want %q", got, tc.want)
			}
		})
	}
}
