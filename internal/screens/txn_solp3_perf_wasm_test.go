// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// TestSOLP3RowUpdateBreakdown3300 separates the action and its subscribed
// rerenders from the explicit extra render in the unchanged SOLP1 harness.
func TestSOLP3RowUpdateBreakdown3300(t *testing.T) {
	a := solp3FlagsApp(t, 3300)
	oldApp, oldCapture := appstate.Default, auditview.CaptureNow
	appstate.Default = a
	auditview.CaptureNow = func() {}
	defer func() { appstate.Default = oldApp; auditview.CaptureNow = oldCapture; uistate.CapturePersistNow(nil) }()
	a.SetDurableWriter(func(revision uint64, done func(appstate.DurableResult)) {
		done(appstate.DurableResult{Revision: revision})
	})
	uistate.CapturePersistNow(func() {})
	uistate.PersistTxFilter(txnfilter.Criteria{PageSize: 25, Sort: "date"})
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Transactions))
	counts := map[string]int{}
	elapsed := map[string]time.Duration{}
	oldObserve := txnPerfObserve
	txnPerfObserve = func(stage string, d time.Duration) { counts[stage]++; elapsed[stage] += d }
	defer func() { txnPerfObserve = oldObserve }()
	id := a.Transactions()[0].ID
	category := a.Categories()[1].ID
	start := time.Now()
	applyTxnManualCategory(id, category)
	action := time.Since(start)
	t.Logf("TCS2 action-only counts=%v durations=%v", counts, elapsed)
	clear(counts)
	clear(elapsed)
	start = time.Now()
	f.Render(ui.CreateElement(Transactions))
	extra := time.Since(start)
	got, ok := a.TransactionByID(id)
	if !ok || got.CategoryID != category {
		t.Fatal("recategorise did not persist row")
	}
	t.Logf("SOLP3 recategorise action+subscribed renders=%.3f ms explicit additional render=%.3f ms stages=%v durations=%v", float64(action.Microseconds())/1000, float64(extra.Microseconds())/1000, counts, elapsed)
}
