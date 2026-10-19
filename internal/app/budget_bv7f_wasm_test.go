// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBV7FRepeatedBudgetUndoIsIncrementalAndReliable(t *testing.T) {
	a, commits := bv6dJournal(t, true)
	budget := a.Budgets()[0]
	start := budget.Limit.Amount
	exports := 0
	persistPerfObserve = func(stage string) {
		if stage == "dataset-export" {
			exports++
		}
	}

	for index, delta := range []int64{500, -250} {
		budget = a.Budgets()[0]
		budget.Limit = money.New(budget.Limit.Amount+delta, budget.Limit.Currency)
		if err := a.PutBudgetAt(budget, time.Now()); err != nil {
			t.Fatalf("edit %d: %v", index+1, err)
		}
		durable := make(chan error, 1)
		a.RequestDurable(func(result appstate.DurableResult) { durable <- result.Err })
		commit := waitSOLP2Commit(t, commits)
		if _, full := commit.values[datasetStoreKey]; full {
			t.Fatalf("edit %d exported the full dataset", index+1)
		}
		bv6dFinish(t, commit)
		if err := <-durable; err != nil {
			t.Fatalf("edit %d durability: %v", index+1, err)
		}

		completed := make(chan error, 1)
		undoLastChangeThen(false, func(err error) { completed <- err })
		undoCommit := waitSOLP2Commit(t, commits)
		if _, full := undoCommit.values[datasetStoreKey]; full {
			t.Fatalf("undo %d exported the full dataset", index+1)
		}
		bv6dFinish(t, undoCommit)
		if err := <-completed; err != nil {
			t.Fatalf("undo %d: %v", index+1, err)
		}
		got := a.Budgets()[0].Limit.Amount
		if got != start {
			t.Fatalf("undo %d left limit at %d minor units, want %d", index+1, got, start)
		}
	}
	if exports != 0 {
		t.Fatalf("budget edits and Undos exported the dataset %d times", exports)
	}
}
