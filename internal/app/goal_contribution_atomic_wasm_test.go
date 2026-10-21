// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workflow"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// contributionUndoHistory exercises the production capture and Undo/Redo paths.
// SQL commits are acknowledged synchronously here; browser durability has its
// separate persistence tests and must never rerun the contribution on retry.
func contributionUndoHistory(t *testing.T, a *appstate.App) {
	t.Helper()
	oldStack, oldSnap := undoStack, lastUndoSnap
	oldCapture, oldPending := undoCaptureCursor, undoPending
	oldApplying, oldPersistence, oldDurable := applyingUndo, undoPersistencePending, undoDurableCursor
	oldApp, oldRev := lastUndoCaptureApp, lastUndoCaptureRev
	t.Cleanup(func() {
		undoStack, lastUndoSnap = oldStack, oldSnap
		undoCaptureCursor, undoPending = oldCapture, oldPending
		applyingUndo, undoPersistencePending, undoDurableCursor = oldApplying, oldPersistence, oldDurable
		lastUndoCaptureApp, lastUndoCaptureRev = oldApp, oldRev
	})
	undoStack = history.NewStack(4 * 1024 * 1024)
	applyingUndo, undoPersistencePending = false, false
	initUndo()
	a.SetDurableWriter(func(revision uint64, done func(appstate.DurableResult)) {
		cursor, err := a.Store().ChangeCursor()
		if err == nil {
			publishDurableUndo(cursor)
		}
		done(appstate.DurableResult{Revision: revision, Err: err})
	})
}

func contributionUndoRequest(t *testing.T, redo bool) error {
	t.Helper()
	called := false
	var result error
	undoLastChangeThen(redo, func(err error) { called, result = true, err })
	if !called {
		t.Fatal("synchronous test durability did not complete Undo/Redo")
	}
	return result
}

func TestGoalContributionGlobalUndoRedoIncludesLedgerAndWorkflowEffects(t *testing.T) {
	goal := contributionHostGoal(100)
	a, _, _ := contributionHostApp(t, goal)
	if err := a.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutWorkflow(workflow.Workflow{ID: "celebrate", Name: "Celebrate", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerGoalReached},
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Plan the trip"}}}); err != nil {
		t.Fatal(err)
	}
	contributionUndoHistory(t, a)
	result, err := a.ContributeToGoal(goal, money.New(900, "USD"), true)
	if err != nil || result.TransactionID == "" || len(a.Tasks()) != 1 || len(a.WorkflowRuns()) != 1 {
		t.Fatalf("contribution did not create complete operation: %+v %v", result, err)
	}
	afterGoal := a.Goals()[0]
	afterTxn, afterTasks, afterRuns := a.Transactions(), a.Tasks(), a.WorkflowRuns()
	if err := contributionUndoRequest(t, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Goals()[0], goal) || len(a.Transactions()) != 0 || len(a.Tasks()) != 0 || len(a.WorkflowRuns()) != 0 {
		t.Fatal("global Undo left part of the contribution or its workflow effects")
	}
	if err := contributionUndoRequest(t, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Goals()[0], afterGoal) || !reflect.DeepEqual(a.Transactions(), afterTxn) || !reflect.DeepEqual(a.Tasks(), afterTasks) || !reflect.DeepEqual(a.WorkflowRuns(), afterRuns) {
		t.Fatal("global Redo lost records or reran contribution workflows")
	}
}

func TestGoalContributionGlobalUndoConflictKeepsWholeOperation(t *testing.T) {
	goal := contributionHostGoal(100)
	a, _, _ := contributionHostApp(t, goal)
	if err := a.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	contributionUndoHistory(t, a)
	if _, err := a.ContributeToGoal(goal, money.New(200, "USD"), true); err != nil {
		t.Fatal(err)
	}
	// Capture the contribution first, then model an intervening edit that was
	// observed by another capture consumer. The checked replay must protect it.
	captureUndoPoint()
	cursor, err := a.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	publishDurableUndo(cursor)
	changed := a.Goals()[0]
	changed.Notes = "Keep the intervening edit"
	if err := a.Store().PutGoal(changed); err != nil {
		t.Fatal(err)
	}
	lastUndoCaptureRev = a.Rev()
	if err := contributionUndoRequest(t, false); err == nil {
		t.Fatal("global Undo overwrote intervening goal metadata")
	}
	if !reflect.DeepEqual(a.Goals()[0], changed) || len(a.Transactions()) != 1 || len(changed.Contributions) != 1 || !undoStack.CanUndo() {
		t.Fatal("conflicting Undo partially reversed the contribution or lost its retry position")
	}
}

func TestGoalContributionHostLedgerFailureRetainsFormAndRetriesOnce(t *testing.T) {
	goal := contributionHostGoal(100)
	goal.TargetAmount = money.New(2000, "USD") // keep this retry below celebration thresholds
	a, edit, notice := contributionHostApp(t, goal)
	account := domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	renderContributionHost(f, goal.ID)
	contributionHostInput(t, f, goal.ID, "2.00", true)
	// The debit source changes after typing. Submitting the rendered USD amount
	// against this EUR account must leave the complete contribution untouched.
	account.Currency = "EUR"
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	rev := a.Rev()
	contributionHostForm(t, f).Submit()
	f.Stabilize()
	if !reflect.DeepEqual(a.Goals()[0], goal) || len(a.Transactions()) != 0 || a.Rev() != rev || notice.Get().Seq != 0 || edit.Get().ID != goal.ID || f.ByRole("alert", "") == nil {
		t.Fatalf("failed ledger posting mutated progress or closed its form: %s", f.Text())
	}
	account.Currency = "USD"
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	contributionHostForm(t, f).Submit()
	f.Stabilize()
	stored := a.Goals()[0]
	if stored.CurrentAmount.Amount != 300 || len(stored.Contributions) != 1 || len(a.Transactions()) != 1 || edit.Get().ID != "" || !notice.Get().Undoable {
		t.Fatalf("corrected retry contributed more than once or failed to close: %s", f.Text())
	}
	if notice.Get().Text == uistate.T("goals.completionPrompt") {
		t.Fatal("partial contribution announced goal completion")
	}
}
