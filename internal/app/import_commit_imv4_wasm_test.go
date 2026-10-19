//go:build js && wasm

package app

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
)

func TestIMV4DurableImportPublishesOneUndoEntry(t *testing.T) {
	for _, failFirst := range []bool{false, true} {
		name := "success"
		if failFirst {
			name = "retry after abort"
		}
		t.Run(name, func(t *testing.T) {
			a, commits := bv6dJournal(t, false)
			beforeCount := len(a.Transactions())
			batch, err := a.Stage(domain.DocCSV, []extract.Row{
				{ID: "one", Date: "2026-10-01", Description: "IMV4 first purchase", Amount: "-1.25"},
				{ID: "two", Date: "2026-10-02", Description: "IMV4 second purchase", Amount: "-2.50"},
			}, a.Accounts()[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			a.CommitImportBatchDurable(batch, "imv4-history", "checkpoint", nil, func(result appstate.CSVImportResult, err error) {
				if result.Imported != 2 {
					t.Errorf("imported=%d", result.Imported)
				}
				finished <- err
			})
			commit := waitSOLP2Commit(t, commits)
			if undoStack.Len() != 0 {
				t.Fatal("undo was published before the strict commit")
			}
			if failFirst {
				commit.done(errors.New("strict transaction aborted"))
				if err := <-finished; err == nil {
					t.Fatal("failed save acknowledged success")
				}
				if undoStack.Len() != 0 {
					t.Fatal("failed save published undo")
				}
				if len(a.Transactions()) != beforeCount+2 {
					t.Fatal("failed durable save lost retryable imported rows")
				}
				a.RequestDurable(func(result appstate.DurableResult) { finished <- result.Err })
				commit = waitSOLP2Commit(t, commits)
			}
			bv6dFinish(t, commit)
			if err := <-finished; err != nil {
				t.Fatal(err)
			}
			if undoStack.Len() != 1 {
				t.Fatalf("undo entries=%d, want one import", undoStack.Len())
			}
			change, ok := undoStack.Undo()
			if !ok {
				t.Fatal("import undo missing")
			}
			transactions, documents := 0, 0
			for _, record := range change.Changes {
				if record.Collection == "transactions" {
					transactions++
				}
				if record.Collection == "documents" && record.ID == "imv4-history" {
					documents++
				}
			}
			if transactions != 2 || documents != 1 {
				t.Fatalf("incomplete undo: transactions=%d documents=%d", transactions, documents)
			}
		})
	}
}
