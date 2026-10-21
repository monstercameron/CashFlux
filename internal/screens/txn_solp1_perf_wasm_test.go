// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestSOLP1Profile3300 exercises the production page and category action in the
// wasm testkit. Timings are evidence, never pass/fail thresholds. Run with
// GOOS=js GOARCH=wasm go test ./internal/screens/ -run TestSOLP1Profile3300 -v.
func TestSOLP1Profile3300(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	ds := store.SampleDataset()
	ds.Transactions = r3pFixture(3300)
	for i := range ds.Transactions {
		ds.Transactions[i].AccountID = ds.Accounts[0].ID
		ds.Transactions[i].CategoryID = ds.Categories[0].ID
	}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	oldApp, oldCapture := appstate.Default, auditview.CaptureNow
	appstate.Default = app
	counts, elapsed := map[string]int{}, map[string]time.Duration{}
	txnPerfObserve = func(stage string, d time.Duration) { counts[stage]++; elapsed[stage] += d }
	defer func() { txnPerfObserve = nil }()
	defer func() { appstate.Default = oldApp; auditview.CaptureNow = oldCapture; uistate.CapturePersistNow(nil) }()
	uistate.PersistTxFilter(txnfilter.Criteria{PageSize: 25, Sort: "date"})
	r3pTime(t, "SOLP1 dataset export", 1, func() {
		if _, err := app.ExportJSONRedacted(); err != nil {
			t.Fatal(err)
		}
	})
	r3pTime(t, "SOLP1 ledger read cold", 1, func() { _ = app.Transactions() })
	f := render.New(t)
	defer f.Cleanup()
	r3pTime(t, "SOLP1 route production mount", 1, func() { f.Render(ui.CreateElement(Transactions)) })
	t.Logf("SOLP1 mount counts=%v durations=%v", counts, elapsed)
	clear(counts)
	clear(elapsed)
	persists, captures := 0, 0
	app.SetDurableWriter(func(revision uint64, done func(appstate.DurableResult)) {
		persists++
		done(appstate.DurableResult{Revision: revision})
	})
	uistate.CapturePersistNow(func() { persists++ })
	auditview.CaptureNow = func() { captures++ }
	r3pTime(t, "SOLP1 production recategorise+subscribed renders", 1, func() {
		applyTxnManualCategory(ds.Transactions[0].ID, ds.Categories[1].ID)
	})
	got, ok := app.TransactionByID(ds.Transactions[0].ID)
	if !ok || got.CategoryID != ds.Categories[1].ID {
		t.Fatalf("category action did not update row: %+v", got)
	}
	t.Logf("SOLP1 work: rows=%d persist requests=%d undo captures=%d", len(ds.Transactions), persists, captures)
	t.Logf("SOLP1 recategorise counts=%v durations=%v", counts, elapsed)
	// Count expensive invalidations, not asynchronous decoration renders or
	// elapsed wall time. Flags/suggestions can settle during a slow CI action.
	if persists != 1 || captures != 1 || counts["filter-sort"] != 1 || counts["category-picker"] != 0 || counts["ledger-counts"] > 1 || counts["summary"] > 1 || counts["placement-txn-table"] != 1 || counts["placement-txn-toolbar"] != 1 {
		t.Fatalf("recategorise exceeded work budget: persists=%d captures=%d stages=%v", persists, captures, counts)
	}
	r3pTime(t, "SOLP1 deferred flags (still unsliced)", 1, func() {
		_ = buildTxnRowFlags(app, uistate.LoadSmartSettings(), time.Monday)
	})
}
