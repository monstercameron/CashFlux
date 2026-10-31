// SPDX-License-Identifier: MIT

package domain_test

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestAccountGroupHasAccount(t *testing.T) {
	tests := []struct {
		name  string
		group domain.AccountGroup
		id    string
		want  bool
	}{
		{"member is found", domain.AccountGroup{AccountIDs: []string{"a1", "a2", "a3"}}, "a2", true},
		{"first member is found", domain.AccountGroup{AccountIDs: []string{"a1", "a2"}}, "a1", true},
		{"last member is found", domain.AccountGroup{AccountIDs: []string{"a1", "a2"}}, "a2", true},
		{"non-member is not found", domain.AccountGroup{AccountIDs: []string{"a1", "a2"}}, "a9", false},
		{"nil membership has no accounts", domain.AccountGroup{}, "a1", false},
		{"empty membership has no accounts", domain.AccountGroup{AccountIDs: []string{}}, "a1", false},
		{"match is case sensitive", domain.AccountGroup{AccountIDs: []string{"abc"}}, "ABC", false},
		{"match is exact, not a prefix", domain.AccountGroup{AccountIDs: []string{"acct-10"}}, "acct-1", false},
		{"blank id does not match real members", domain.AccountGroup{AccountIDs: []string{"a1"}}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.HasAccount(tt.id); got != tt.want {
				t.Errorf("HasAccount(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestGoalLinkedAccountIDs(t *testing.T) {
	tests := []struct {
		name string
		goal domain.Goal
		want []string
	}{
		{"no links yields nothing", domain.Goal{}, nil},
		{"legacy single account only", domain.Goal{AccountID: "legacy"}, []string{"legacy"}},
		{"multi-select only keeps order", domain.Goal{AccountIDs: []string{"b", "a"}}, []string{"b", "a"}},
		{
			"legacy account comes first, then the multi-select list",
			domain.Goal{AccountID: "legacy", AccountIDs: []string{"b", "c"}},
			[]string{"legacy", "b", "c"},
		},
		{
			"legacy account already in the list is not repeated",
			domain.Goal{AccountID: "b", AccountIDs: []string{"a", "b", "c"}},
			[]string{"b", "a", "c"},
		},
		{
			"duplicates inside the list collapse to one",
			domain.Goal{AccountIDs: []string{"a", "a", "b", "a"}},
			[]string{"a", "b"},
		},
		{
			"blank ids are dropped",
			domain.Goal{AccountID: "", AccountIDs: []string{"", "a", ""}},
			[]string{"a"},
		},
		{"only blank ids yields nothing", domain.Goal{AccountID: "", AccountIDs: []string{"", ""}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.goal.LinkedAccountIDs()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LinkedAccountIDs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGoalLinkedAccountIDsDoesNotMutateGoal(t *testing.T) {
	g := domain.Goal{AccountID: "x", AccountIDs: []string{"a", "x", "a"}}
	_ = g.LinkedAccountIDs()
	if want := []string{"a", "x", "a"}; !reflect.DeepEqual(g.AccountIDs, want) {
		t.Errorf("AccountIDs changed to %v, want %v", g.AccountIDs, want)
	}
}

func TestGoalEffectiveKind(t *testing.T) {
	tests := []struct {
		name          string
		kind          domain.GoalKind
		want          domain.GoalKind
		wantFinancial bool
	}{
		{"unset kind resolves to financial", "", domain.GoalKindFinancial, true},
		{"financial stays financial", domain.GoalKindFinancial, domain.GoalKindFinancial, true},
		{"checklist is kept", domain.GoalKindChecklist, domain.GoalKindChecklist, false},
		{"milestone is kept", domain.GoalKindMilestone, domain.GoalKindMilestone, false},
		{"habit is kept", domain.GoalKindHabit, domain.GoalKindHabit, false},
		{"unknown kind is passed through unchanged", domain.GoalKind("mystery"), domain.GoalKind("mystery"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := domain.Goal{Kind: tt.kind}
			if got := g.EffectiveKind(); got != tt.want {
				t.Errorf("EffectiveKind() = %q, want %q", got, tt.want)
			}
			if got := g.IsFinancial(); got != tt.wantFinancial {
				t.Errorf("IsFinancial() = %v, want %v", got, tt.wantFinancial)
			}
		})
	}
}

func TestAccountByMask(t *testing.T) {
	acct := func(id, mask string) domain.Account { return domain.Account{ID: id, Mask: mask} }

	tests := []struct {
		name     string
		accounts []domain.Account
		digits   string
		wantID   string
		wantOK   bool
	}{
		{"nil accounts finds nothing", nil, "6500", "", false},
		{"empty accounts finds nothing", []domain.Account{}, "6500", "", false},
		{"single match", []domain.Account{acct("a1", "6500"), acct("a2", "1111")}, "6500", "a1", true},
		{"mask typed with punctuation still matches", []domain.Account{acct("a1", "xxxx-6500")}, "6500", "a1", true},
		{"asterisk-prefixed mask matches", []domain.Account{acct("a1", "*6500")}, "6500", "a1", true},
		{"long mask matches on its last four digits", []domain.Account{acct("a1", "1234 5678 9012 6500")}, "6500", "a1", true},
		{"no account has that mask", []domain.Account{acct("a1", "1111"), acct("a2", "2222")}, "6500", "", false},
		{
			"two accounts sharing a mask is ambiguous",
			[]domain.Account{acct("a1", "6500"), acct("a2", "*6500"), acct("a3", "1111")},
			"6500", "", false,
		},
		{"accounts without a mask never match a blank query", []domain.Account{acct("a1", ""), acct("a2", "")}, "", "", false},
		{"blank query ignores accounts that have a mask", []domain.Account{acct("a1", "6500")}, "", "", false},
		{"mask with no digits is treated as no mask", []domain.Account{acct("a1", "****")}, "", "", false},
		{"query must be the digits only, not the raw mask text", []domain.Account{acct("a1", "*6500")}, "*6500", "", false},
		{"partial digits do not match", []domain.Account{acct("a1", "6500")}, "650", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := domain.AccountByMask(tt.accounts, tt.digits)
			if ok != tt.wantOK {
				t.Fatalf("AccountByMask(%q) ok = %v, want %v", tt.digits, ok, tt.wantOK)
			}
			// The account returned alongside ok=false is unspecified, so only
			// check identity when a match is reported.
			if ok && got.ID != tt.wantID {
				t.Errorf("AccountByMask(%q) id = %q, want %q", tt.digits, got.ID, tt.wantID)
			}
		})
	}
}
