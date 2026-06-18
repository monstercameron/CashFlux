package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	for _, tr := range a.Transactions() {
		if tr.ID == "t1" && tr.MemberID != "m2" {
			t.Errorf("transaction member = %q, want m2", tr.MemberID)
		}
	}
}
