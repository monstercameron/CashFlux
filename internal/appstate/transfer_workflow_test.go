// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// seedTwoAccounts creates a checking and a savings account and returns both.
func seedTwoAccounts(t *testing.T, a *App) (checking, savings domain.Account) {
	t.Helper()
	checking = domain.Account{
		ID: "chk1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"), // $1,000.00
	}
	savings = domain.Account{
		ID: "sav1", Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: money.New(0, "USD"),
	}
	if err := a.PutAccount(checking); err != nil {
		t.Fatalf("seedTwoAccounts PutAccount checking: %v", err)
	}
	if err := a.PutAccount(savings); err != nil {
		t.Fatalf("seedTwoAccounts PutAccount savings: %v", err)
	}
	return checking, savings
}

// TestActionTransferProducesTwoLegs verifies that executing an ActionTransfer
// workflow via RunWorkflow creates the out-leg (negative, on checking) and the
// in-leg (positive, on savings).
func TestActionTransferProducesTwoLegs(t *testing.T) {
	a := newApp(t, false)
	checking, savings := seedTwoAccounts(t, a)

	wf := workflow.Workflow{
		ID: "wf-transfer", Name: "Monthly savings", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerScheduled},
		Actions: []workflow.Action{{
			Kind:                  workflow.ActionTransfer,
			TransferFromAccountID: checking.ID,
			TransferToAccountID:   savings.ID,
			TransferAmount:        20000, // $200.00 in cents
			DedupeKey:             "test:wf-transfer:2026-06",
		}},
	}

	run, err := a.RunWorkflow(wf, false)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if !run.Matched {
		t.Fatal("expected workflow to match")
	}
	if len(run.Effects) != 1 || run.Effects[0].Kind != workflow.ActionTransfer {
		t.Fatalf("expected one transfer effect, got %+v", run.Effects)
	}

	// Both legs should now appear in the transaction log.
	txns := a.Transactions()
	var outLegs, inLegs int
	for _, tx := range txns {
		if tx.AccountID == checking.ID && tx.Amount.IsNegative() {
			outLegs++
		}
		if tx.AccountID == savings.ID && tx.Amount.IsPositive() {
			inLegs++
		}
	}
	if outLegs != 1 {
		t.Errorf("expected 1 out-leg on checking, got %d (txns=%+v)", outLegs, txns)
	}
	if inLegs != 1 {
		t.Errorf("expected 1 in-leg on savings, got %d (txns=%+v)", inLegs, txns)
	}
}

// TestActionTransferDedupePreventsDuplication verifies that running the same
// ActionTransfer workflow twice with the same DedupeKey only transfers once.
func TestActionTransferDedupePreventsDuplication(t *testing.T) {
	a := newApp(t, false)
	checking, savings := seedTwoAccounts(t, a)

	wf := workflow.Workflow{
		ID: "wf-dedup", Name: "Dedup transfer", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerScheduled},
		Actions: []workflow.Action{{
			Kind:                  workflow.ActionTransfer,
			TransferFromAccountID: checking.ID,
			TransferToAccountID:   savings.ID,
			TransferAmount:        5000,
			DedupeKey:             "pyf:wf-dedup:2026-06",
		}},
	}

	// First run: should execute.
	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Second run: same DedupeKey — should be skipped.
	if _, err := a.RunWorkflow(wf, false); err != nil {
		t.Fatalf("second run: %v", err)
	}

	// Count legs: only one pair should exist.
	txns := a.Transactions()
	var outLegs int
	for _, tx := range txns {
		if tx.AccountID == checking.ID && tx.Amount.IsNegative() {
			outLegs++
		}
	}
	if outLegs != 1 {
		t.Errorf("dedupe failed: expected 1 out-leg, got %d", outLegs)
	}
}
