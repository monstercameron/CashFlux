// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func findGoal(t *testing.T, a *App, id string) domain.Goal {
	t.Helper()
	for _, g := range a.Goals() {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("goal %q not found", id)
	return domain.Goal{}
}

func hasTxn(a *App, id string) bool {
	for _, tx := range a.Transactions() {
		if tx.ID == id {
			return true
		}
	}
	return false
}

func TestContributeRecordsUndoRemovesLedger(t *testing.T) {
	a := newApp(t, false)
	acct := domain.Account{ID: "acc1", Name: "Savings", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"}
	if err := a.PutAccount(acct); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	g := domain.Goal{ID: "g1", Name: "Trip", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CurrentAmount: money.New(0, "USD"), TargetAmount: money.New(100000, "USD"), AccountID: "acc1"}
	if err := a.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	// Contribute with the "also move money" ledger posting.
	res, err := a.ContributeToGoal(g, money.New(25000, "USD"), true)
	if err != nil {
		t.Fatalf("ContributeToGoal: %v", err)
	}
	if res.TransactionID == "" {
		t.Fatal("expected a ledger transaction id")
	}

	g2 := findGoal(t, a, "g1")
	if g2.CurrentAmount.Amount != 25000 {
		t.Errorf("current = %d, want 25000", g2.CurrentAmount.Amount)
	}
	if len(g2.Contributions) != 1 {
		t.Fatalf("contributions = %d, want 1", len(g2.Contributions))
	}
	if g2.Contributions[0].TxnID != res.TransactionID {
		t.Errorf("logged txn = %q, want %q", g2.Contributions[0].TxnID, res.TransactionID)
	}
	if !hasTxn(a, res.TransactionID) {
		t.Error("ledger txn should exist after contribute")
	}

	// Undo it: progress reverses AND the ledger entry is removed.
	undone, ok, err := a.UndoLastContribution(g2)
	if err != nil || !ok {
		t.Fatalf("UndoLastContribution: ok=%v err=%v", ok, err)
	}
	if undone.Amount != 25000 {
		t.Errorf("undone amount = %d, want 25000", undone.Amount)
	}
	g3 := findGoal(t, a, "g1")
	if g3.CurrentAmount.Amount != 0 {
		t.Errorf("after undo current = %d, want 0", g3.CurrentAmount.Amount)
	}
	if len(g3.Contributions) != 0 {
		t.Errorf("after undo contributions = %d, want 0", len(g3.Contributions))
	}
	if hasTxn(a, res.TransactionID) {
		t.Error("ledger txn should be deleted on undo")
	}

	// Nothing left to undo.
	if _, ok, _ := a.UndoLastContribution(g3); ok {
		t.Error("expected ok=false when there's nothing to undo")
	}
}

func TestResetGoalToZero(t *testing.T) {
	a := newApp(t, false)
	g := domain.Goal{ID: "g2", Name: "Fund", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CurrentAmount: money.New(50000, "USD"), TargetAmount: money.New(100000, "USD")}
	g = g.RecordContribution(domain.GoalContribution{Amount: money.New(50000, "USD")})
	if err := a.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	if err := a.ResetGoalToZero(g); err != nil {
		t.Fatalf("ResetGoalToZero: %v", err)
	}
	g2 := findGoal(t, a, "g2")
	if g2.CurrentAmount.Amount != 0 {
		t.Errorf("current = %d, want 0", g2.CurrentAmount.Amount)
	}
	if len(g2.Contributions) != 0 {
		t.Errorf("contributions = %d, want 0", len(g2.Contributions))
	}
}
