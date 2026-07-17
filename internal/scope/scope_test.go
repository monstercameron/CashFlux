// SPDX-License-Identifier: MIT

package scope_test

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/scope"
)

// noInstitution is the stub institutionOf accessor used when tests do not care
// about institution-based filtering. It mirrors what callers should pass before
// Account.Institution exists on the domain type.
func noInstitution(_ domain.Account) string { return "" }

// institutionOf is a stub that reads institution from Account.Custom["inst"].
// It simulates the eventual Account.Institution field for test purposes only;
// real callers will pass func(a domain.Account) string { return a.Institution }.
func institutionOf(a domain.Account) string {
	if a.Custom == nil {
		return ""
	}
	v, _ := a.Custom["inst"].(string)
	return v
}

// acct constructs a minimal Account for testing.
func acct(id, ownerID string, typ domain.AccountType, archived bool, institution string) domain.Account {
	a := domain.Account{
		ID:       id,
		OwnerID:  ownerID,
		Type:     typ,
		Archived: archived,
	}
	if institution != "" {
		a.Custom = map[string]any{"inst": institution}
	}
	return a
}

// txn constructs a minimal Transaction for testing.
func txn(id, accountID string) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: accountID}
}

// ---- ReportScope.IsAll ----

func TestIsAll(t *testing.T) {
	tests := []struct {
		name string
		s    scope.ReportScope
		want bool
	}{
		{"zero value", scope.ReportScope{}, true},
		{"institutions set", scope.ReportScope{Institutions: []string{"Chase"}}, false},
		{"owners set", scope.ReportScope{Owners: []string{"alice"}}, false},
		{"types set", scope.ReportScope{Types: []domain.AccountType{domain.TypeChecking}}, false},
		{"accountIDs set", scope.ReportScope{AccountIDs: []string{"a1"}}, false},
		{"all dims set", scope.ReportScope{
			Institutions: []string{"x"},
			Owners:       []string{"y"},
			Types:        []domain.AccountType{domain.TypeSavings},
			AccountIDs:   []string{"z"},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.s.IsAll()
			if got != tt.want {
				t.Errorf("IsAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---- ResolveScope ----

func TestResolveScope_EmptyScope_AllNonArchived(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", "bob", domain.TypeSavings, false, ""),
		acct("arch", "alice", domain.TypeChecking, true, ""), // archived — excluded
	}
	got := scope.ResolveScope(accounts, scope.ReportScope{}, noInstitution)
	want := []string{"a1", "a2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_InstitutionFilter_CaseInsensitive(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, "Chase"),
		acct("a2", "bob", domain.TypeSavings, false, "bank of america"),
		acct("a3", "carol", domain.TypeCash, false, ""),
	}
	s := scope.ReportScope{Institutions: []string{"chase"}} // lowercase query
	got := scope.ResolveScope(accounts, s, institutionOf)
	want := []string{"a1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_InstitutionFilter_MultipleMatches(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, "Chase"),
		acct("a2", "bob", domain.TypeSavings, false, "CHASE"), // same institution, different case
		acct("a3", "carol", domain.TypeCash, false, "Wells Fargo"),
	}
	s := scope.ReportScope{Institutions: []string{"Chase", "Wells Fargo"}}
	got := scope.ResolveScope(accounts, s, institutionOf)
	want := []string{"a1", "a2", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_OwnerFilter(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", "bob", domain.TypeSavings, false, ""),
		acct("a3", domain.GroupOwnerID, domain.TypeSavings, false, ""),
	}
	s := scope.ReportScope{Owners: []string{"alice"}}
	got := scope.ResolveScope(accounts, s, noInstitution)
	// Alice's perspective = her accounts PLUS the household's shared ones — a3 is
	// group-owned, so it belongs to every member's scope. Bob's individual a2 does not.
	want := []string{"a1", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_OwnerFilter_SharedScopeAccounts(t *testing.T) {
	// The account data model marks sharing via Account.Scope while OwnerID records
	// the CREATING member (e.g. "Joint Checking" created by Marcus). A member scope
	// must include those shared accounts for every member, or the member who didn't
	// create them sees an empty household (the /reports member-switcher bug).
	joint := acct("joint", "marcus", domain.TypeChecking, false, "")
	joint.Scope = domain.ScopeShared
	own401k := acct("k401", "marcus", domain.TypeInvestment, false, "")
	own401k.Scope = domain.ScopeIndividual
	priyaBiz := acct("biz", "priya", domain.TypeChecking, false, "")
	priyaBiz.Scope = domain.ScopeIndividual

	accounts := []domain.Account{joint, own401k, priyaBiz}
	got := scope.ResolveScope(accounts, scope.ReportScope{Owners: []string{"priya"}}, noInstitution)
	// Priya sees her own business account AND the shared joint account — not
	// Marcus's individual 401(k).
	want := []string{"biz", "joint"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_OwnerFilter_GroupOwner(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", domain.GroupOwnerID, domain.TypeSavings, false, ""),
		acct("a3", domain.GroupOwnerID, domain.TypeChecking, false, ""),
	}
	s := scope.ReportScope{Owners: []string{domain.GroupOwnerID}}
	got := scope.ResolveScope(accounts, s, noInstitution)
	want := []string{"a2", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_TypeFilter(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", "bob", domain.TypeSavings, false, ""),
		acct("a3", "carol", domain.TypeCreditCard, false, ""),
	}
	s := scope.ReportScope{Types: []domain.AccountType{domain.TypeChecking, domain.TypeCreditCard}}
	got := scope.ResolveScope(accounts, s, noInstitution)
	want := []string{"a1", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_MultiDim_AND(t *testing.T) {
	// alice has a Chase checking and a Chase savings.
	// bob has a Chase checking.
	// Only alice's Chase checking satisfies owner=alice AND inst=Chase AND type=checking.
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, "Chase"),
		acct("a2", "alice", domain.TypeSavings, false, "Chase"),
		acct("a3", "bob", domain.TypeChecking, false, "Chase"),
	}
	s := scope.ReportScope{
		Institutions: []string{"Chase"},
		Owners:       []string{"alice"},
		Types:        []domain.AccountType{domain.TypeChecking},
	}
	got := scope.ResolveScope(accounts, s, institutionOf)
	want := []string{"a1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_AccountIDs_Union(t *testing.T) {
	// Scope filters for owner=alice, but a3 (owned by bob) is in AccountIDs.
	// a3 should appear in the result as an additive union; archived a4 should not.
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", "alice", domain.TypeSavings, false, ""),
		acct("a3", "bob", domain.TypeSavings, false, ""), // not alice's, but listed in AccountIDs
		acct("a4", "bob", domain.TypeCash, true, ""),     // archived — union must not resurrect it
	}
	s := scope.ReportScope{
		Owners:     []string{"alice"},
		AccountIDs: []string{"a3", "a4"}, // a4 is archived, should still be excluded
	}
	got := scope.ResolveScope(accounts, s, noInstitution)
	want := []string{"a1", "a2", "a3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_ArchivedExcluded(t *testing.T) {
	accounts := []domain.Account{
		acct("live", "alice", domain.TypeChecking, false, ""),
		acct("dead", "alice", domain.TypeChecking, true, ""),
	}
	// Even with IsAll() the archived account must not appear.
	got := scope.ResolveScope(accounts, scope.ReportScope{}, noInstitution)
	want := []string{"live"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestResolveScope_EmptyAccounts(t *testing.T) {
	got := scope.ResolveScope(nil, scope.ReportScope{}, noInstitution)
	if len(got) != 0 {
		t.Errorf("expected empty result for empty accounts, got %v", got)
	}
}

func TestResolveScope_NoMatch(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
	}
	s := scope.ReportScope{Owners: []string{"nobody"}}
	got := scope.ResolveScope(accounts, s, noInstitution)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// ---- ApplyScopeToTxns ----

func TestApplyScopeToTxns(t *testing.T) {
	txns := []domain.Transaction{
		txn("t1", "a1"),
		txn("t2", "a2"),
		txn("t3", "a1"),
		txn("t4", "a3"),
	}
	ids := []string{"a1", "a3"}
	got := scope.ApplyScopeToTxns(txns, ids)
	if len(got) != 3 {
		t.Fatalf("expected 3 transactions, got %d: %v", len(got), got)
	}
	for _, tx := range got {
		if tx.AccountID != "a1" && tx.AccountID != "a3" {
			t.Errorf("unexpected accountID %q in result", tx.AccountID)
		}
	}
}

func TestApplyScopeToTxns_EmptyIDs(t *testing.T) {
	txns := []domain.Transaction{txn("t1", "a1")}
	got := scope.ApplyScopeToTxns(txns, nil)
	if len(got) != 0 {
		t.Errorf("expected nil/empty result for empty ids, got %v", got)
	}
}

func TestApplyScopeToTxns_NoMatch(t *testing.T) {
	txns := []domain.Transaction{txn("t1", "a1")}
	got := scope.ApplyScopeToTxns(txns, []string{"zzz"})
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestApplyScopeToTxns_PreservesOrder(t *testing.T) {
	txns := []domain.Transaction{
		txn("t3", "a1"),
		txn("t1", "a1"),
		txn("t2", "a1"),
	}
	got := scope.ApplyScopeToTxns(txns, []string{"a1"})
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].ID != "t3" || got[1].ID != "t1" || got[2].ID != "t2" {
		t.Errorf("order not preserved: %v", got)
	}
}

// ---- ApplyScopeToAccounts ----

func TestApplyScopeToAccounts(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", "bob", domain.TypeSavings, false, ""),
		acct("a3", "carol", domain.TypeCash, false, ""),
	}
	ids := []string{"a1", "a3"}
	got := scope.ApplyScopeToAccounts(accounts, ids)
	if len(got) != 2 {
		t.Fatalf("expected 2 accounts, got %d: %v", len(got), got)
	}
	if got[0].ID != "a1" || got[1].ID != "a3" {
		t.Errorf("unexpected accounts: %v", got)
	}
}

func TestApplyScopeToAccounts_EmptyIDs(t *testing.T) {
	accounts := []domain.Account{acct("a1", "alice", domain.TypeChecking, false, "")}
	got := scope.ApplyScopeToAccounts(accounts, nil)
	if len(got) != 0 {
		t.Errorf("expected nil/empty result for empty ids, got %v", got)
	}
}

func TestApplyScopeToAccounts_PreservesOrder(t *testing.T) {
	accounts := []domain.Account{
		acct("z1", "alice", domain.TypeChecking, false, ""),
		acct("a1", "alice", domain.TypeSavings, false, ""),
		acct("m1", "alice", domain.TypeCash, false, ""),
	}
	ids := []string{"z1", "a1", "m1"}
	got := scope.ApplyScopeToAccounts(accounts, ids)
	if len(got) != 3 || got[0].ID != "z1" || got[1].ID != "a1" || got[2].ID != "m1" {
		t.Errorf("order not preserved: %v", got)
	}
}

// ---- Integration: ResolveScope → Apply* round-trip ----

func TestRoundTrip_ResolveThenApply(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, "Chase"),
		acct("a2", "alice", domain.TypeSavings, false, "Chase"),
		acct("a3", "bob", domain.TypeChecking, false, "Wells Fargo"),
		acct("arch", "alice", domain.TypeChecking, true, "Chase"),
	}
	txns := []domain.Transaction{
		txn("t1", "a1"),
		txn("t2", "a2"),
		txn("t3", "a3"),
		txn("tarch", "arch"),
	}

	s := scope.ReportScope{
		Institutions: []string{"Chase"},
		Owners:       []string{"alice"},
	}

	ids := scope.ResolveScope(accounts, s, institutionOf)
	// Only a1 and a2 match: Chase AND alice; arch is archived.
	if !reflect.DeepEqual(ids, []string{"a1", "a2"}) {
		t.Fatalf("ResolveScope = %v, want [a1 a2]", ids)
	}

	filteredAccts := scope.ApplyScopeToAccounts(accounts, ids)
	if len(filteredAccts) != 2 {
		t.Errorf("ApplyScopeToAccounts: got %d, want 2", len(filteredAccts))
	}

	filteredTxns := scope.ApplyScopeToTxns(txns, ids)
	if len(filteredTxns) != 2 {
		t.Errorf("ApplyScopeToTxns: got %d, want 2", len(filteredTxns))
	}
	for _, tx := range filteredTxns {
		if tx.AccountID != "a1" && tx.AccountID != "a2" {
			t.Errorf("unexpected txn accountID %q", tx.AccountID)
		}
	}
}

func TestMerge(t *testing.T) {
	lens := scope.ReportScope{Owners: []string{"m1"}}
	tests := []struct {
		name  string
		lens  scope.ReportScope
		local scope.ReportScope
		want  scope.ReportScope
	}{
		{"both empty", scope.ReportScope{}, scope.ReportScope{}, scope.ReportScope{}},
		{"lens only", lens, scope.ReportScope{}, scope.ReportScope{Owners: []string{"m1"}}},
		{"local only", scope.ReportScope{}, scope.ReportScope{Types: []domain.AccountType{domain.TypeChecking}},
			scope.ReportScope{Types: []domain.AccountType{domain.TypeChecking}}},
		{"local narrows inside lens", lens, scope.ReportScope{Types: []domain.AccountType{domain.TypeChecking}},
			scope.ReportScope{Owners: []string{"m1"}, Types: []domain.AccountType{domain.TypeChecking}}},
		{"local owners win over lens", lens, scope.ReportScope{Owners: []string{"m2"}},
			scope.ReportScope{Owners: []string{"m2"}}},
		{"institutions fall back to lens", scope.ReportScope{Institutions: []string{"Chase"}}, scope.ReportScope{},
			scope.ReportScope{Institutions: []string{"Chase"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scope.Merge(tc.lens, tc.local)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("scope.Merge(%+v, %+v) = %+v, want %+v", tc.lens, tc.local, got, tc.want)
			}
		})
	}
}

// TestResolveScope_AccountIDsOnly_Restricts locks the QA CF-01/UX-03 fix: a
// scope whose ONLY non-empty part is AccountIDs restricts to exactly those
// accounts. The old code ran the dimensional loop with no dimensions — which
// matches everything — so "Specific accounts: one account" resolved to the
// whole household and every report figure stayed unchanged.
func TestResolveScope_AccountIDsOnly_Restricts(t *testing.T) {
	accounts := []domain.Account{
		acct("a1", "alice", domain.TypeChecking, false, ""),
		acct("a2", "bob", domain.TypeSavings, false, ""),
		acct("a3", "bob", domain.TypeCash, false, ""),
		acct("a4", "bob", domain.TypeCash, true, ""), // archived — never returned
	}
	s := scope.ReportScope{AccountIDs: []string{"a2", "a4"}}
	got := scope.ResolveScope(accounts, s, noInstitution)
	want := []string{"a2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AccountIDs-only scope resolved to %v, want %v (restriction, not the whole household)", got, want)
	}
}
