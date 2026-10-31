// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTV8HBulkFeedbackUsesCommittedChanges(t *testing.T) {
	for _, mode := range []string{"three", "partial", "zero", "metadata only", "unchanged", "write failure", "durable failure"} {
		t.Run(mode, func(t *testing.T) {
			a := sol1EditApp(t)
			old := appstate.Default
			appstate.Default = a
			defer func() { appstate.Default = old }()
			txn, _ := a.TransactionByID("sol1-record")
			ids := []string{txn.ID, "tv8h-two", "tv8h-three", "missing", txn.ID}
			for _, id := range ids[1:3] {
				row := txn
				row.ID = id
				if err := a.Store().PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			}
			f := render.New(t)
			defer f.Cleanup()
			var notice state.Atom[uistate.Notice]
			f.Render(ui.CreateElement(func() ui.Node { notice = uistate.UseNotice(); return Div() }))
			notice.Set(uistate.Notice{})
			var complete func(appstate.DurableResult)
			var revision uint64
			a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { revision, complete = r, done })
			err := CommitUndoableCounted(a, func(n int) string { return fmt.Sprintf("Changed %d transactions", n) }, ids, func() error {
				switch mode {
				case "write failure":
					return errors.New("store: sqlite3: disk I/O error")
				case "zero":
					_, err := a.DeleteTransactionsBulkExpanded([]string{"missing"})
					return err
				case "metadata only":
					return a.SetKV("tv8h:derived", "updated")
				case "unchanged":
					return a.PutTransactionsAtomic([]domain.Transaction{txn})
				case "partial":
					_, err := a.DeleteTransactionsBulkExpanded([]string{txn.ID, txn.ID, "missing"})
					return err
				default:
					_, err := a.DeleteTransactionsBulkExpanded(ids)
					return err
				}
			}, nil)
			if mode == "write failure" {
				if err == nil || complete != nil {
					t.Fatal("failed write requested durability")
				}
			} else {
				if err != nil || complete == nil {
					t.Fatalf("write = %v; callback missing = %v", err, complete == nil)
				}
				if notice.Get().Text != "" {
					t.Fatalf("premature feedback: %+v", notice.Get())
				}
				result := appstate.DurableResult{Revision: revision}
				if mode == "durable failure" {
					result.Err = errors.New("quota exceeded")
				}
				complete(result)
			}
			n := notice.Get()
			switch mode {
			case "zero", "metadata only", "unchanged":
				if n.Text != "" || n.Undoable {
					t.Fatalf("no-op posted success: %+v", n)
				}
			case "write failure", "durable failure":
				if !n.Err || n.Undoable || strings.Contains(n.Text, "sqlite") {
					t.Fatalf("dishonest failure: %+v", n)
				}
			default:
				want := "Changed 3 transactions"
				if mode == "partial" {
					want = "Changed 1 transactions"
				}
				if n.Err || !n.Undoable || !n.Durable || n.Text != want {
					t.Fatalf("committed feedback: %+v; want %q", n, want)
				}
			}
		})
	}
}

func TestTV8HReplayAcknowledgesOnlyDurableCompletion(t *testing.T) {
	for _, mode := range []string{"success", "failure", "scope changed"} {
		t.Run(mode, func(t *testing.T) {
			a := sol1EditApp(t)
			old := appstate.Default
			appstate.Default = a
			defer func() { appstate.Default = old }()
			f := render.New(t)
			defer f.Cleanup()
			var notice state.Atom[uistate.Notice]
			f.Render(ui.CreateElement(func() ui.Node { notice = uistate.UseNotice(); return Div() }))
			notice.Set(uistate.Notice{})
			var complete func(error)
			calls := 0
			CommitUndoableReplay(a, "Change undone.", func(done func(error)) { complete = done }, func(err error) { calls++ })
			if notice.Get().Text != "" || calls != 0 {
				t.Fatal("undo acknowledged early")
			}
			var err error
			if mode == "failure" {
				err = errors.New("failed")
			}
			if mode == "scope changed" {
				appstate.Default = nil
			}
			complete(err)
			complete(err)
			n := notice.Get()
			if mode == "scope changed" {
				if calls != 0 || n.Text != "" {
					t.Fatal("undo crossed scope")
				}
			} else if calls != 1 {
				t.Fatalf("completed %d times", calls)
			}
			if mode == "success" {
				if !n.Durable || n.Undoable || n.Text != "Change undone." {
					t.Fatalf("undo = %+v", n)
				}
			} else if n.Text != "" {
				t.Fatalf("false undo success: %+v", n)
			}
		})
	}
}

func TestTV8HBulkBarReportsOnlyDurableWrites(t *testing.T) {
	for _, mode := range []string{"three", "zero", "write failure", "durable failure"} {
		t.Run(mode, func(t *testing.T) {
			a := sol1EditApp(t)
			old := appstate.Default
			appstate.Default = a
			defer func() { appstate.Default = old }()
			original, _ := a.TransactionByID("sol1-record")
			selection := map[string]bool{}
			for _, id := range []string{original.ID, "tv8h-two", "tv8h-three"} {
				row := original
				row.ID = id
				row.Reviewed = mode == "zero"
				if err := a.Store().PutTransaction(row); err != nil {
					t.Fatal(err)
				}
				selection[id] = true
			}
			if mode == "write failure" {
				a.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
			}
			var complete func(appstate.DurableResult)
			var rev uint64
			a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { rev, complete = r, done })
			f := render.New(t)
			defer f.Cleanup()
			var notice state.Atom[uistate.Notice]
			f.Render(ui.CreateElement(func() ui.Node {
				notice = uistate.UseNotice()
				selected := uistate.UseTxnSelection()
				seeded := ui.UseState(false)
				if !seeded.Get() {
					selected.Set(selection)
					seeded.Set(true)
				}
				return ui.CreateElement(txnBulkBarWidget, txnBulkBarProps{App: a})
			}))
			notice.Set(uistate.Notice{})
			button := bulk2ByTestID(f, "button", "bulk-mark-reviewed")
			if button == nil {
				t.Fatal("missing bulk review action")
			}
			button.Click()
			if mode == "zero" || mode == "write failure" {
				// Nothing to change says so in a notice that is not a success, so the only text allowed is that one.
				if complete != nil || notice.Get().Undoable || (notice.Get().Text != "" && !notice.Get().Err && notice.Get().Text != uistate.T("transactions.bulkNothing")) {
					t.Fatalf("zero/failed write posted success: %+v", notice.Get())
				}
				return
			}
			if complete == nil || notice.Get().Text != "" {
				t.Fatal("bulk acknowledged before durable completion")
			}
			result := appstate.DurableResult{Revision: rev}
			if mode == "durable failure" {
				result.Err = errors.New("quota exhausted")
			}
			complete(result)
			got := notice.Get()
			if mode == "durable failure" {
				if !got.Err || got.Undoable {
					t.Fatalf("failed bulk offered Undo: %+v", got)
				}
			} else if got.Err || !got.Durable || !got.Undoable || !strings.Contains(got.Text, "3") {
				t.Fatalf("three-row bulk feedback = %+v", got)
			}
		})
	}
}
