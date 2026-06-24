// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestPutCustomFieldDefRejectsInvalid covers the validation-rejection branch: a
// definition missing its key/label/type is refused with validate.Issues.
func TestPutCustomFieldDefRejectsInvalid(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCustomFieldDef(customfields.Def{ID: "d1", EntityType: "account"}); err == nil {
		t.Error("an incomplete custom-field definition should be rejected")
	}
	if len(a.CustomFieldDefs()) != 0 {
		t.Error("a rejected definition must not be stored")
	}
}

// TestReassignOwnerAllEntityTypes covers every move loop (account, budget, goal,
// transaction) and the individual-member target path (the existing test only
// reassigns an account+goal to the group owner).
func TestReassignOwnerAllEntityTypes(t *testing.T) {
	a := newApp(t, false)
	for _, m := range []domain.Member{{ID: "m1", Name: "Alex"}, {ID: "m2", Name: "Bo"}} {
		if err := a.PutMember(m); err != nil {
			t.Fatalf("PutMember %s: %v", m.ID, err)
		}
	}
	mustPut := func(name string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	mustPut("account", a.PutAccount(domain.Account{
		ID: "a1", Name: "Alex Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: "m1", Scope: domain.ScopeIndividual,
	}))
	mustPut("budget", a.PutBudget(domain.Budget{
		ID: "b1", Name: "Alex Food", CategoryID: "food", Period: domain.PeriodMonthly,
		Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: money.New(10000, "USD"),
	}))
	mustPut("goal", a.PutGoal(domain.Goal{
		ID: "g1", Name: "Alex Trip", OwnerID: "m1", Scope: domain.ScopeIndividual,
		TargetAmount: money.New(100000, "USD"),
	}))
	mustPut("transaction", a.PutTransaction(domain.Transaction{
		ID: "t1", AccountID: "a1", Desc: "Lunch", Amount: money.New(-1200, "USD"),
		Date: time.Now(), MemberID: "m1",
	}))

	moved, err := a.ReassignOwner("m1", "m2")
	if err != nil {
		t.Fatalf("ReassignOwner: %v", err)
	}
	if moved != 4 {
		t.Errorf("moved = %d, want 4 (account, budget, goal, transaction)", moved)
	}

	// Every moved entity now belongs to m2, individually.
	for _, ac := range a.Accounts() {
		if ac.ID == "a1" && (ac.OwnerID != "m2" || ac.Scope != domain.ScopeIndividual) {
			t.Errorf("account owner/scope = %q/%q, want m2/individual", ac.OwnerID, ac.Scope)
		}
	}
	for _, b := range a.Budgets() {
		if b.ID == "b1" && (b.OwnerID != "m2" || b.Scope != domain.ScopeIndividual) {
			t.Errorf("budget owner/scope = %q/%q, want m2/individual", b.OwnerID, b.Scope)
		}
	}
	for _, g := range a.Goals() {
		if g.ID == "g1" && (g.OwnerID != "m2" || g.Scope != domain.ScopeIndividual) {
			t.Errorf("goal owner/scope = %q/%q, want m2/individual", g.OwnerID, g.Scope)
		}
	}
	for _, tr := range a.Transactions() {
		if tr.ID == "t1" && tr.MemberID != "m2" {
			t.Errorf("transaction member = %q, want m2", tr.MemberID)
		}
	}
}

func TestSetDefaultMemberAndDeleteAfterReassign(t *testing.T) {
	a := newApp(t, false)
	for _, m := range []domain.Member{
		{ID: "m1", Name: "Alex", IsDefault: true},
		{ID: "m2", Name: "Bo"},
	} {
		if err := a.PutMember(m); err != nil {
			t.Fatalf("PutMember %s: %v", m.ID, err)
		}
	}
	if err := a.SetDefaultMember("m2"); err != nil {
		t.Fatalf("SetDefaultMember: %v", err)
	}
	defaults := 0
	for _, m := range a.Members() {
		if m.IsDefault {
			defaults++
			if m.ID != "m2" {
				t.Fatalf("default member = %q, want m2", m.ID)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("default member count = %d, want 1", defaults)
	}
	if got := a.DefaultMemberID(); got != "m2" {
		t.Fatalf("DefaultMemberID = %q, want m2", got)
	}
	if got := a.MemberForNewTransaction(domain.Account{OwnerID: "m1", Scope: domain.ScopeIndividual}); got != "m1" {
		t.Fatalf("individual account transaction member = %q, want m1", got)
	}
	if got := a.MemberForNewTransaction(domain.Account{OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); got != "m2" {
		t.Fatalf("shared account transaction member = %q, want default m2", got)
	}

	mustPut := func(name string, err error) {
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	mustPut("account", a.PutAccount(domain.Account{
		ID: "a1", Name: "Alex Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: "m1", Scope: domain.ScopeIndividual,
	}))
	mustPut("budget", a.PutBudget(domain.Budget{
		ID: "b1", Name: "Alex Food", CategoryID: "food", Period: domain.PeriodMonthly,
		Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: money.New(10000, "USD"),
	}))
	mustPut("goal", a.PutGoal(domain.Goal{
		ID: "g1", Name: "Alex Trip", OwnerID: "m1", Scope: domain.ScopeIndividual,
		TargetAmount: money.New(100000, "USD"),
	}))
	mustPut("transaction", a.PutTransaction(domain.Transaction{
		ID: "t1", AccountID: "a1", Desc: "Lunch", Amount: money.New(-1200, "USD"),
		Date: time.Now(), MemberID: "m1",
	}))

	moved, err := a.DeleteMemberAfterReassign("m1", "m2")
	if err != nil {
		t.Fatalf("DeleteMemberAfterReassign: %v", err)
	}
	if moved != 4 {
		t.Fatalf("moved = %d, want 4", moved)
	}
	for _, m := range a.Members() {
		if m.ID == "m1" {
			t.Fatal("deleted member m1 still present")
		}
	}
	for _, ac := range a.Accounts() {
		if ac.OwnerID == "m1" {
			t.Fatalf("account %s orphaned to deleted owner", ac.ID)
		}
	}
	for _, b := range a.Budgets() {
		if b.OwnerID == "m1" {
			t.Fatalf("budget %s orphaned to deleted owner", b.ID)
		}
	}
	for _, g := range a.Goals() {
		if g.OwnerID == "m1" {
			t.Fatalf("goal %s orphaned to deleted owner", g.ID)
		}
	}
	for _, tx := range a.Transactions() {
		if tx.MemberID == "m1" {
			t.Fatalf("transaction %s orphaned to deleted member", tx.ID)
		}
	}
	byOwner, err := ledger.NetByOwner(a.Accounts(), a.Transactions(), currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("NetByOwner: %v", err)
	}
	if _, ok := byOwner["m1"]; ok {
		t.Fatalf("deleted member should not appear in net-worth rollups: %v", byOwner)
	}
	if !byOwner["m2"].Equal(money.New(-1200, "USD")) {
		t.Fatalf("m2 rollup = %v, want -1200 USD", byOwner["m2"])
	}
}
