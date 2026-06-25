// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// seedGoalWithAccount creates a checking account, a savings account (to serve
// as the goal's destination), and a goal linked to the savings account.
func seedGoalWithAccount(t *testing.T, a *App) (checking, savings domain.Account, goal domain.Goal) {
	t.Helper()
	checking = domain.Account{
		ID: "chk-pyf", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(500000, "USD"), // $5,000
	}
	savings = domain.Account{
		ID: "sav-pyf", Name: "Goal Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}
	goal = domain.Goal{
		ID:            "goal-pyf",
		Name:          "Vacation Fund",
		Scope:         domain.ScopeIndividual,
		OwnerID:       "m1",
		TargetAmount:  money.New(300000, "USD"), // $3,000
		CurrentAmount: money.New(0, "USD"),
		AccountID:     savings.ID,
	}
	if err := a.PutAccount(checking); err != nil {
		t.Fatalf("seedGoalWithAccount: PutAccount checking: %v", err)
	}
	if err := a.PutAccount(savings); err != nil {
		t.Fatalf("seedGoalWithAccount: PutAccount savings: %v", err)
	}
	if err := a.PutGoal(goal); err != nil {
		t.Fatalf("seedGoalWithAccount: PutGoal: %v", err)
	}
	return
}

// TestCreateWorkflowFromGoal_HappyPath checks the normal case: a goal with a
// linked account and an eligible funding account produces a persisted,
// enabled, scheduled workflow with a single ActionTransfer.
func TestCreateWorkflowFromGoal_HappyPath(t *testing.T) {
	a := newApp(t, false)
	checking, savings, goal := seedGoalWithAccount(t, a)

	const monthly = 25000 // $250/mo in cents
	wf, err := a.CreateWorkflowFromGoal(goal.ID, monthly)
	if err != nil {
		t.Fatalf("CreateWorkflowFromGoal: unexpected error: %v", err)
	}

	// Verify workflow shape.
	if wf.ID == "" {
		t.Error("workflow ID must not be empty")
	}
	if !wf.Enabled {
		t.Error("workflow must be enabled")
	}
	if wf.Trigger.Kind != workflow.TriggerScheduled {
		t.Errorf("trigger kind: want %q, got %q", workflow.TriggerScheduled, wf.Trigger.Kind)
	}
	if wf.Trigger.Cadence != domain.CadenceMonthly {
		t.Errorf("cadence: want %q, got %q", domain.CadenceMonthly, wf.Trigger.Cadence)
	}
	if len(wf.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(wf.Actions))
	}
	act := wf.Actions[0]
	if act.Kind != workflow.ActionTransfer {
		t.Errorf("action kind: want %q, got %q", workflow.ActionTransfer, act.Kind)
	}
	if act.TransferFromAccountID != checking.ID {
		t.Errorf("from account: want %q, got %q", checking.ID, act.TransferFromAccountID)
	}
	if act.TransferToAccountID != savings.ID {
		t.Errorf("to account: want %q, got %q", savings.ID, act.TransferToAccountID)
	}
	if act.TransferAmount != monthly {
		t.Errorf("transfer amount: want %d, got %d", monthly, act.TransferAmount)
	}
	if act.DedupeKey == "" {
		t.Error("DedupeKey must not be empty")
	}

	// Verify it was persisted.
	var found bool
	for _, w := range a.Workflows() {
		if w.ID == wf.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("workflow was not persisted in the store")
	}
}

// TestCreateWorkflowFromGoal_NoLinkedAccount ensures a clear error is returned
// when the goal has no linked account.
func TestCreateWorkflowFromGoal_NoLinkedAccount(t *testing.T) {
	a := newApp(t, false)
	// Create a checking account (funding source) but a goal with no AccountID.
	acc := domain.Account{
		ID: "chk-x", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"),
	}
	g := domain.Goal{
		ID: "goal-nolink", Name: "Floaty Goal", Scope: domain.ScopeIndividual,
		OwnerID:      "m1",
		TargetAmount: money.New(100000, "USD"),
		// AccountID intentionally left empty.
	}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	_, err := a.CreateWorkflowFromGoal(g.ID, 10000)
	if err == nil {
		t.Fatal("expected an error when goal has no linked account, got nil")
	}
}

// TestCreateWorkflowFromGoal_NoFundingAccount ensures a clear error is returned
// when no eligible funding account exists.
func TestCreateWorkflowFromGoal_NoFundingAccount(t *testing.T) {
	a := newApp(t, false)
	// Create a savings account (the goal destination), but no other asset account
	// to use as a funding source.
	savOnly := domain.Account{
		ID: "sav-only", Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}
	g := domain.Goal{
		ID: "goal-nofund", Name: "Lonely Goal", Scope: domain.ScopeIndividual,
		OwnerID:      "m1",
		TargetAmount: money.New(100000, "USD"),
		AccountID:    savOnly.ID,
	}
	if err := a.PutAccount(savOnly); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutGoal(g); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	_, err := a.CreateWorkflowFromGoal(g.ID, 10000)
	if err == nil {
		t.Fatal("expected an error when no funding account exists, got nil")
	}
}

// TestCreateWorkflowFromGoal_ZeroAmount ensures a positive amount is required.
func TestCreateWorkflowFromGoal_ZeroAmount(t *testing.T) {
	a := newApp(t, false)
	_, _, goal := seedGoalWithAccount(t, a)

	_, err := a.CreateWorkflowFromGoal(goal.ID, 0)
	if err == nil {
		t.Fatal("expected an error for zero monthly amount, got nil")
	}
}

// TestPickFundingAccount_PrefersChecking verifies that pickFundingAccount
// prefers checking/debit accounts over savings/investment accounts when both
// exist as eligible options.
func TestPickFundingAccount_PrefersChecking(t *testing.T) {
	sav := domain.Account{ID: "sav", Class: domain.ClassAsset, Type: domain.TypeSavings}
	chk := domain.Account{ID: "chk", Class: domain.ClassAsset, Type: domain.TypeChecking}
	accounts := []domain.Account{sav, chk}
	got := pickFundingAccount(accounts, "goal-acc")
	if got != "chk" {
		t.Errorf("expected checking account to be preferred, got %q", got)
	}
}

// TestPickFundingAccount_ExcludesGoalAccount ensures the goal's destination
// account is never chosen as the funding source.
func TestPickFundingAccount_ExcludesGoalAccount(t *testing.T) {
	goalAcc := domain.Account{ID: "goal-acc", Class: domain.ClassAsset, Type: domain.TypeChecking}
	other := domain.Account{ID: "other", Class: domain.ClassAsset, Type: domain.TypeSavings}
	got := pickFundingAccount([]domain.Account{goalAcc, other}, "goal-acc")
	if got != "other" {
		t.Errorf("expected other account, got %q", got)
	}
}

// TestPickFundingAccount_SkipsLiabilities verifies that liability accounts are
// never selected as the funding source.
func TestPickFundingAccount_SkipsLiabilities(t *testing.T) {
	cc := domain.Account{ID: "cc", Class: domain.ClassLiability, Type: domain.TypeCreditCard}
	got := pickFundingAccount([]domain.Account{cc}, "goal-acc")
	if got != "" {
		t.Errorf("expected empty string (no eligible account), got %q", got)
	}
}
