// SPDX-License-Identifier: MIT

package appstate

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func atomicContributionApp(t *testing.T) (*App, domain.Goal) {
	t.Helper()
	a := newApp(t, false)
	t.Cleanup(func() { _ = a.Store().Close() })
	goal := contributionSafetyGoal(100)
	if err := a.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	if err := a.PutAccount(domain.Account{ID: "bank", Name: "Bank", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	return a, goal
}

func TestContributionLedgerFailureCanRetryWithoutDoubleProgress(t *testing.T) {
	a, goal := atomicContributionApp(t)
	calls := 0
	a.OnTxnMutated(func() { calls++ })
	rev := a.Rev()
	cursor, err := a.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.ContributeToGoalFrom(goal, money.New(200, "USD"), true, "missing")
	if err == nil || result != (ContributeResult{}) {
		t.Fatalf("failed requested debit returned success facts: %+v %v", result, err)
	}
	nextCursor, cursorErr := a.Store().ChangeCursor()
	if !reflect.DeepEqual(findGoal(t, a, goal.ID), goal) || len(a.Transactions()) != 0 || calls != 0 || a.Rev() != rev || cursorErr != nil || nextCursor != cursor {
		t.Fatal("rejected debit partially saved progress, history, ledger or effects")
	}
	result, err = a.ContributeToGoalFrom(goal, money.New(200, "USD"), true, "bank")
	stored := findGoal(t, a, goal.ID)
	if err != nil || result.TransactionID == "" || stored.CurrentAmount.Amount != 300 || len(stored.Contributions) != 1 || len(a.Transactions()) != 1 || calls != 1 {
		t.Fatalf("retry did not contribute once: %+v %+v %v", result, stored, err)
	}
	for _, request := range []struct {
		post bool
		from string
	}{{false, "missing"}, {true, ""}} {
		if result, err := a.ContributeToGoalFrom(goal, money.New(100, "USD"), request.post, request.from); err != nil || result.TransactionID != "" {
			t.Fatalf("explicit memo-only request failed: %+v %v", result, err)
		}
	}
	if findGoal(t, a, goal.ID).CurrentAmount.Amount != 500 || len(a.Transactions()) != 1 || calls != 1 {
		t.Fatal("memo-only contribution posted ledger effects")
	}
}

func TestContributionCheckedCommitPreservesInterveningGoalAndExistingDebit(t *testing.T) {
	for _, conflict := range []string{"goal", "transaction"} {
		t.Run(conflict, func(t *testing.T) {
			a, before := atomicContributionApp(t)
			after := before
			after.CurrentAmount = money.New(300, "USD")
			after = after.RecordContribution(domain.GoalContribution{Amount: money.New(200, "USD"), TxnID: "debit"})
			txn := domain.Transaction{ID: "debit", AccountID: "bank", Date: time.Now(), Amount: money.New(-200, "USD")}
			if conflict == "goal" {
				changed := before
				changed.Notes = "Intervening edit"
				if err := a.PutGoal(changed); err != nil {
					t.Fatal(err)
				}
			} else {
				existing := txn
				existing.Desc = "Existing unrelated row"
				if err := a.PutTransaction(existing); err != nil {
					t.Fatal(err)
				}
			}
			baseline, err := a.Store().Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			rev := a.Rev()
			cursor, err := a.Store().ChangeCursor()
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			a.OnTxnMutated(func() { calls++ })
			if err := a.commitGoalContribution(before, after, &txn); err == nil {
				t.Fatal("checked contribution overwrote a conflicting record")
			}
			got, err := a.Store().Snapshot()
			nextCursor, cursorErr := a.Store().ChangeCursor()
			if err != nil || cursorErr != nil || nextCursor != cursor || a.Rev() != rev || calls != 0 || !reflect.DeepEqual(got, baseline) {
				t.Fatal("failed contribution escaped its atomic boundary")
			}
		})
	}
}

func TestContributionEffectsObserveCommittedGoalAndRetainOrder(t *testing.T) {
	a, goal := atomicContributionApp(t)
	var events []string
	a.Notifier = func(message string) { events = append(events, message) }
	for _, w := range []workflow.Workflow{
		{ID: "added", Name: "Added", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "transaction"}}},
		{ID: "reached", Name: "Reached", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerGoalReached}, Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "goal"}}},
	} {
		if err := a.PutWorkflow(w); err != nil {
			t.Fatal(err)
		}
	}
	a.OnTxnMutated(func() {
		events = append(events, "observer")
		stored := findGoal(t, a, goal.ID)
		if stored.CurrentAmount.Amount != 1000 || len(stored.Contributions) != 1 || len(a.Transactions()) != 1 || stored.Contributions[0].TxnID != a.Transactions()[0].ID {
			t.Fatal("observer saw a half-saved contribution")
		}
	})
	if _, err := a.ContributeToGoalFrom(goal, money.New(900, "USD"), true, "missing"); err == nil || len(events) != 0 {
		t.Fatal("failed contribution emitted effects")
	}
	result, err := a.ContributeToGoal(goal, money.New(900, "USD"), true)
	if err != nil || !result.BecameComplete || !reflect.DeepEqual(events, []string{"transaction", "observer", "goal"}) || len(a.WorkflowRuns()) != 2 {
		t.Fatalf("contribution effect order changed: %v %+v %v", events, result, err)
	}
}

func TestContributionUndoDeletesRelationshipsBeforeNotifying(t *testing.T) {
	a, goal := atomicContributionApp(t)
	result, err := a.ContributeToGoal(goal, money.New(200, "USD"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutTxnLink(domain.TxnLink{ID: "group", Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{result.TransactionID, "other"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store().PutDebtPaymentLink(domain.DebtPaymentLink{ID: "debt", TransactionIDs: []string{result.TransactionID}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.OnTxnMutated(func() {
		calls++
		links, err := a.Store().ListTxnLinks()
		debt, debtErr := a.Store().ListDebtPaymentLinks()
		stored := findGoal(t, a, goal.ID)
		if err != nil || debtErr != nil || len(links) != 0 || len(debt) != 0 || len(a.Transactions()) != 0 || len(stored.Contributions) != 0 || stored.CurrentAmount.Amount != 100 {
			t.Fatal("undo observer saw partial goal/ledger/relationship state")
		}
	})
	if amount, ok, err := a.UndoLastContribution(goal); err != nil || !ok || amount != money.New(200, "USD") || calls != 1 {
		t.Fatalf("atomic undo failed: %v %v %v, calls %d", amount, ok, err, calls)
	}
	if _, ok, err := a.UndoLastContribution(goal); err != nil || ok || calls != 1 {
		t.Fatal("completed undo repeated its effects")
	}
}
