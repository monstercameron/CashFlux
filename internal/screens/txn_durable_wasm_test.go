// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSOLP2EditAcknowledgesOnlyDurableCompletion(t *testing.T) {
	a := sol1EditApp(t)
	old := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = old })
	var complete func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { revision = r; complete = done })
	closed := 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: a, TxnID: "sol1-record", OnDone: func() { closed++ }}))
	f.ByID("txn-edit-amount").Input("8.50")
	f.ByID("txn-edit-form").Submit()
	if complete == nil || closed != 0 || f.ByID("txn-edit-form").Attr("aria-busy") != "true" {
		t.Fatal("edit acknowledged before commit or omitted pending state")
	}
	complete(appstate.DurableResult{Revision: revision, Err: errors.New("quota exhausted")})
	if closed != 0 || f.ByID("txn-edit-form") == nil {
		t.Fatal("failed save closed the edit form")
	}
	f.ByID("txn-edit-form").Submit()
	complete(appstate.DurableResult{Revision: revision})
	if closed != 1 {
		t.Fatalf("durable save emitted %d close actions", closed)
	}
}

func TestSOLP2UnmountedEditDoesNotCloseAnotherEditor(t *testing.T) {
	a := sol1EditApp(t)
	old := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = old })
	var complete func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { revision, complete = r, done })
	closed := 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: a, TxnID: "sol1-record", OnDone: func() { closed++ }}))
	f.ByID("txn-edit-amount").Input("8.50")
	f.ByID("txn-edit-form").Submit()
	if complete == nil {
		t.Fatal("edit did not start a durable save")
	}
	f.Rerender(Div("Another editor"))
	complete(appstate.DurableResult{Revision: revision})
	if closed != 0 {
		t.Fatal("an unmounted edit closed the current editor")
	}
}

func TestSOLP2ReplacementEditDoesNotInheritPendingSave(t *testing.T) {
	a := sol1EditApp(t)
	old := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = old })
	second, ok := a.TransactionByID("sol1-record")
	if !ok {
		t.Fatal("missing fixture transaction")
	}
	second.ID = "solp2-next-record"
	if err := a.PutTransaction(second); err != nil {
		t.Fatal(err)
	}
	var complete func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { revision, complete = r, done })
	closed := 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: a, TxnID: "sol1-record", OnDone: func() { closed++ }}))
	f.ByID("txn-edit-amount").Input("8.50")
	f.ByID("txn-edit-form").Submit()
	if complete == nil {
		t.Fatal("edit did not request durability")
	}
	f.Rerender(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: a, TxnID: second.ID, OnDone: func() { closed++ }}))
	if f.ByID("txn-edit-form").Attr("aria-busy") == "true" {
		t.Fatal("replacement editor inherited the previous row's pending state")
	}
	complete(appstate.DurableResult{Revision: revision})
	if closed != 0 {
		t.Fatal("old save completion closed the replacement editor")
	}
}

func TestSOLP2CategoryPendingRowAndUndoWaitForCommit(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	a := appstate.Default
	oldCapture := auditview.CaptureNow
	auditview.CaptureNow = func() {}
	t.Cleanup(func() { auditview.CaptureNow = oldCapture })
	var complete func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { revision = r; complete = done })
	f := render.New(t)
	defer f.Cleanup()
	var notice state.Atom[uistate.Notice]
	p := r4cRow("countline-expense")
	t.Cleanup(func() { setTxnSaveStatus(p.ID, txnSaveIdle) })
	root := func() ui.Node { notice = uistate.UseNotice(); return Table(Tbody(ui.CreateElement(txnFrameRow, &p))) }
	f.Render(ui.CreateElement(root))
	before := notice.Get()
	t.Cleanup(func() { notice.Set(before) })
	applyTxnManualCategory(p.ID, "countline-other-category")
	if complete == nil {
		t.Fatal("category write did not request a durable result")
	}
	row := byTestID(f, "tr", "txn-row-"+p.ID)
	if row == nil || row.Attr("aria-busy") != "true" || notice.Get().Undoable {
		t.Fatal("row omitted pending state or advertised Undo before commit")
	}
	complete(appstate.DurableResult{Revision: revision})
	if !notice.Get().Undoable {
		t.Fatal("durable category change did not advertise Undo")
	}
}

func TestSOLP2SplitAndBulkUndoWaitForCommit(t *testing.T) {
	a := sol1EditApp(t)
	old := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = old })
	for _, id := range []string{"split-one", "split-two"} {
		if err := a.Store().PutCategory(domain.Category{ID: id, Name: id, Kind: domain.KindExpense}); err != nil {
			t.Fatal(err)
		}
	}
	txn, _ := a.TransactionByID("sol1-record")
	txn.Splits = []domain.CategorySplit{{CategoryID: "split-one", Amount: money.New(-275, "USD")}, {CategoryID: "split-two", Amount: money.New(-400, "USD")}}
	if err := a.Store().PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	var complete func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { revision = r; complete = done })
	closed := 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionSplitForm, TransactionSplitFormProps{TxnID: txn.ID, OnDone: func() { closed++ }}))
	f.ByID(SplitModalFormID).Submit()
	if complete == nil || closed != 0 || f.ByID(SplitModalFormID).Attr("aria-busy") != "true" {
		t.Fatal("split acknowledged before commit")
	}
	complete(appstate.DurableResult{Revision: revision, Err: errors.New("commit failed")})
	if closed != 0 {
		t.Fatal("failed split closed the modal")
	}
	f.ByID(SplitModalFormID).Submit()
	complete(appstate.DurableResult{Revision: revision})
	if closed != 1 {
		t.Fatal("durable split did not close")
	}
	published := 0
	snapshot := uistate.BulkSnapshot{Label: "Changed one transaction", Prior: []domain.Transaction{txn}}
	setTxnBulkUndoAfterCommit(a, snapshot, func(uistate.BulkSnapshot) { published++ })
	if published != 0 {
		t.Fatal("bulk Undo published before durability")
	}
	complete(appstate.DurableResult{Revision: revision})
	if published != 1 {
		t.Fatal("durable bulk Undo was not published")
	}
}
