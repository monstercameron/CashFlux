// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func int7ResetView(t *testing.T) {
	t.Helper()
	mode := state.NewGlobalAtom("transactions:viewMode", uistate.TxnViewTable)
	previousMode := mode.Get()
	mode.Set(uistate.TxnViewTable)
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousLens := lens.Get()
	lens.Set(scope.ReportScope{})
	t.Cleanup(func() { mode.Set(previousMode); lens.Set(previousLens) })
}

func int7FactsBands(f *render.Fixture) []*render.QueryNode {
	var bands []*render.QueryNode
	for _, node := range f.AllByTag("div") {
		if id := node.Attr("data-testid"); id == "txn-table-count-facts" || id == "txn-count-visible-facts" {
			bands = append(bands, node)
		}
	}
	return bands
}

func TestINT7HostDerivesOncePerRender(t *testing.T) {
	_, _ = tv7ScopeApp(t)
	int7ResetView(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	previousObserve := txnPerfObserve
	defer func() { txnPerfObserve = previousObserve }()
	counts := map[string]int{}
	txnPerfObserve = func(stage string, _ time.Duration) { counts[stage]++ }
	for _, tt := range []struct {
		name, text, categories string
		scopeAny               bool
	}{
		{name: "whole ledger"},
		{name: "search", text: "Coffee"},
		{name: "clear search"},
		{name: "multi category", categories: "food", scopeAny: true},
		{name: "no results", text: "no-such-transaction"},
	} {
		clear(counts)
		filter := uistate.CurrentTxFilter()
		filter.Text, filter.Categories, filter.ScopeAny = tt.text, tt.categories, tt.scopeAny
		uistate.PersistTxFilter(filter)
		state.NewGlobalAtom("transactions:filter", filter).Set(filter)
		f.Render(ui.CreateElement(Transactions))
		f.Stabilize()
		if counts["scope-derive"] != 1 || counts["filter-sort"] != 1 {
			t.Fatalf("%s repeated derivation: %v", tt.name, counts)
		}
		bands := int7FactsBands(f)
		if len(bands) != 1 || bands[0].Attr("data-testid") != "txn-table-count-facts" {
			t.Fatalf("%s: facts bands=%d", tt.name, len(bands))
		}
		if tt.name == "whole ledger" || tt.name == "clear search" {
			if strings.Contains(bands[0].Text(), "unpaired") || !strings.Contains(bands[0].Text(), "$480.00") {
				t.Fatalf("collapsed transfer changed facts: %s", bands[0].Text())
			}
			if byTestID(f, "tr", "txn-row-transfer-in") != nil {
				t.Fatal("display did not fold the receiving leg")
			}
		}
	}
}

func TestINT7FactsBandMovesWithViewAndKeepsScopeDetails(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	int7ResetView(t)
	// A missing conversion must remain visible even when the amount filter omits
	// that row from the table. The perspective must move with the facts too.
	foreign := rows[0]
	foreign.ID, foreign.Desc, foreign.Amount = "foreign", "Foreign", money.New(-100, "CHF")
	if err := app.Store().PutTransaction(foreign); err != nil {
		t.Fatal(err)
	}
	filter := uistate.CurrentTxFilter()
	filter.AmountMin = "1"
	uistate.PersistTxFilter(filter)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Transactions))
	uistate.SetActiveScope(scope.ReportScope{Owners: []string{"marcus"}})
	f.Stabilize()
	for _, mode := range []string{uistate.TxnViewTable, uistate.TxnViewCalendar, uistate.TxnViewTable} {
		if mode == uistate.TxnViewCalendar {
			toggle := byTestID(f, "button", "txn-view-calendar")
			if toggle == nil {
				t.Fatal("calendar toggle missing")
			}
			toggle.Click()
		} else if byTestID(f, "div", "txn-table-count-facts") == nil {
			toggle := byTestID(f, "button", "txn-view-ledger")
			if toggle == nil {
				t.Fatal("ledger toggle missing")
			}
			toggle.Click()
		}
		f.Stabilize()
		bands := int7FactsBands(f)
		if len(bands) != 1 {
			t.Fatalf("%s: got %d facts bands: %s", mode, len(bands), f.Text())
		}
		t.Logf("mode=%s band=%q text=%s", mode, bands[0].Attr("data-testid"), bands[0].Text())
		want := "txn-table-count-facts"
		if mode == uistate.TxnViewCalendar {
			want = "txn-count-visible-facts"
		}
		if bands[0].Attr("data-testid") != want || !strings.Contains(bands[0].Text(), "CHF") || !strings.Contains(bands[0].Text(), "Marcus Hartley") {
			t.Fatalf("%s: facts id=%q want=%q text=%q", mode, bands[0].Attr("data-testid"), want, bands[0].Text())
		}
	}
}

func TestINT7TableRejectsStaleSnapshotAndCollapsedFallback(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	int7ResetView(t)
	filter := uistate.CurrentTxFilter()
	stale := &txntoolbar.Snapshot{Revision: app.Rev(), Scope: txnDerivedScope(app, txnfilter.Criteria{Text: "Coffee"})}
	f := render.New(t)
	defer f.Cleanup()
	// Pass only the transfer's outgoing display leg. A fallback summary over it
	// invents an orphan; the current full ledger has the counterpart.
	f.Render(txnTableScopedCountFacts(app, filter, []domain.Transaction{rows[2]}, "USD", nil, txnTableFactsContext{Snapshot: stale}))
	bands := int7FactsBands(f)
	if len(bands) != 1 || strings.Contains(bands[0].Text(), "unpaired") || !strings.Contains(bands[0].Text(), "$480.00") {
		t.Fatalf("stale/collapsed facts: %s", f.Text())
	}
}
