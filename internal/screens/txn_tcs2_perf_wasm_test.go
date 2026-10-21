// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTCS2CommitBoundary(t *testing.T) {
	for _, mode := range []string{"success", "write failure", "durable failure", "scope changed"} {
		t.Run(mode, func(t *testing.T) {
			a := sol1EditApp(t)
			old, oldCapture := appstate.Default, auditview.CaptureNow
			appstate.Default = a
			defer func() { appstate.Default = old; auditview.CaptureNow = oldCapture }()
			f := render.New(t)
			defer f.Cleanup()
			var notice state.Atom[uistate.Notice]
			f.Render(ui.CreateElement(func() ui.Node { notice = uistate.UseNotice(); return Div() }))
			notice.Set(uistate.Notice{})
			order := []string{}
			auditview.CaptureNow = func() { order = append(order, "capture") }
			var complete func(appstate.DurableResult)
			var rev uint64
			a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) {
				order = append(order, "request")
				rev = r
				complete = done
			})
			completions := 0
			err := CommitUndoable(a, "Updated one", []string{"sol1-record"}, func() error {
				order = append(order, "write")
				if mode == "write failure" {
					return errors.New("refused")
				}
				txn, _ := a.TransactionByID("sol1-record")
				txn.Note = "changed"
				return a.PutTransactionsAtomic([]domain.Transaction{txn})
			}, func(err error) {
				completions++
				if err != nil {
					uistate.PostNotice("Failed once", true)
				}
			})
			if notice.Get().Undoable {
				t.Fatal("advertised Undo before durability")
			}
			if mode == "write failure" {
				if err == nil || complete != nil || completions != 1 {
					t.Fatal("write failure requested durability or completed twice")
				}
				return
			}
			if err != nil || !reflect.DeepEqual(order, []string{"capture", "write", "request"}) || completions != 0 {
				t.Fatalf("boundary order: %v err=%v completions=%d", order, err, completions)
			}
			result := appstate.DurableResult{Revision: rev}
			if mode == "durable failure" {
				result.Err = errors.New("quota exhausted")
			}
			if mode == "scope changed" {
				appstate.Default = nil
			}
			complete(result)
			complete(result)
			if mode == "scope changed" {
				if completions != 0 || notice.Get().Undoable {
					t.Fatal("completion crossed App scope")
				}
				return
			}
			if completions != 1 || notice.Get().Undoable != (mode == "success") {
				t.Fatalf("wrong completion: %d notice=%+v", completions, notice.Get())
			}
		})
	}
}

func TestTCS2CategoryPathsUseLiveReviewConfirmation(t *testing.T) {
	for _, path := range []string{"row", "chip", "merchant"} {
		t.Run(path, func(t *testing.T) {
			a := sol1EditApp(t)
			old := appstate.Default
			appstate.Default = a
			defer func() { appstate.Default = old }()
			txn, _ := a.TransactionByID("sol1-record")
			txn.Tags = []string{reviewqueue.ReviewTag, "keep"}
			txn.Reviewed = true
			txn.Splits = []domain.CategorySplit{{CategoryID: "old", Amount: txn.Amount}}
			if err := a.Store().PutTransaction(txn); err != nil {
				t.Fatal(err)
			}
			stale := txn
			txn.Note = "new note"
			if err := a.Store().PutTransaction(txn); err != nil {
				t.Fatal(err)
			}
			a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { done(appstate.DurableResult{Revision: r}) })
			switch path {
			case "row":
				applyTxnManualCategory(txn.ID, "chosen")
			case "chip":
				applyCatSuggestion(a, stale, "chosen")
			case "merchant":
				applyCatSuggestionToMerchant(a, stale, "chosen")
			}
			got, _ := a.TransactionByID(txn.ID)
			if got.CategoryID != "chosen" || !got.Reviewed || len(got.Splits) != 0 || !reflect.DeepEqual(got.Tags, []string{"keep"}) || got.Note != "new note" {
				t.Fatalf("confirmation lost facts: %+v", got)
			}
		})
	}
}

func TestTCS2SplitPreservesLiveFields(t *testing.T) {
	a := sol1EditApp(t)
	old := appstate.Default
	appstate.Default = a
	defer func() { appstate.Default = old }()
	txn, _ := a.TransactionByID("sol1-record")
	txn.Splits = []domain.CategorySplit{{CategoryID: "one", Amount: money.New(-275, "USD")}, {CategoryID: "two", Amount: money.New(-400, "USD")}}
	if err := a.Store().PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { done(appstate.DurableResult{Revision: r}) })
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionSplitForm, TransactionSplitFormProps{TxnID: txn.ID}))
	live := txn
	live.Note = "arrived while editing"
	live.Attachments = []domain.AttachmentRef{{ArtifactID: "receipt"}}
	if err := a.Store().PutTransaction(live); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	f.ByID(SplitModalFormID).Submit()
	t.Logf("TCS2 split submit action %.3f ms", float64(time.Since(start).Microseconds())/1000)
	got, _ := a.TransactionByID(txn.ID)
	if got.Note != live.Note || !reflect.DeepEqual(got.Attachments, live.Attachments) {
		t.Fatal("split restored stale non-split fields")
	}
}

func TestTCS2WarmRouteAndWriteProfile3300(t *testing.T) {
	a := solp3FlagsApp(t, 3300)
	old, oldCapture, oldObserve := appstate.Default, auditview.CaptureNow, txnPerfObserve
	t.Cleanup(func() {
		txnPerfObserve = oldObserve
		appstate.Default = old
		auditview.CaptureNow = oldCapture
	})
	appstate.Default = a
	auditview.CaptureNow = func() {}
	a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { done(appstate.DurableResult{Revision: r}) })
	uistate.PersistTxFilter(txnfilter.Criteria{PageSize: 25, Sort: "date"})
	for i := 0; i < 3; i++ {
		func() {
			f := render.New(t)
			defer func() {
				txnPerfObserve = nil
				f.Render(ui.Fragment())
				f.Stabilize()
				f.Cleanup()
			}()
			counts := map[string]int{}
			txnPerfObserve = func(stage string, _ time.Duration) { counts[stage]++ }
			start := time.Now()
			f.Render(ui.CreateElement(Transactions))
			mount := time.Since(start)
			t.Logf("TCS2 mount[%d] %.3f ms counts=%v", i, float64(mount.Microseconds())/1000, counts)
			clear(counts)
			row := a.Transactions()[0]
			category := a.Categories()[i%2].ID
			start = time.Now()
			applyTxnManualCategory(row.ID, category)
			elapsed := time.Since(start)
			t.Logf("TCS2 write[%d] %.3f ms counts=%v", i, float64(elapsed.Microseconds())/1000, counts)
			if counts["filter-sort"] != 1 || counts["placement-txn-table"] != 1 || counts["placement-txn-toolbar"] != 1 {
				t.Fatalf("redundant work: %v", counts)
			}
		}()
	}
}
