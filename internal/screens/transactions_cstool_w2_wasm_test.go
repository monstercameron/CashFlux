// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	"github.com/monstercameron/CashFlux/internal/txnview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestCSTOOLW2SummaryRejectsPreviousCollapsedRows(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	previous := txnDerivedViewCache
	txnDerivedViewCache = txnview.Cache{}
	defer func() { txnDerivedViewCache = previous }()
	// The filter has cleared while the host still supplies the prior coffee row.
	// Reusing that row as a summary fallback under the new scope was TC4-11.
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnToolbarWidget, txnToolbarProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}, Shown: rows[:1]}))
	spent := byTestID(f, "span", "txn-count-spent-value")
	if spent == nil || spent.Text() != "$220.75" {
		t.Fatalf("current scope uses stale summary: %s", f.Text())
	}
	for _, node := range f.AllByTag("span") {
		if strings.Contains(node.Text(), "unpaired") {
			t.Fatalf("collapsed rows invented an orphan: %s", node.Text())
		}
	}
}

func TestCSTOOLW2ToolbarKeepsZeroResultsAndRecovery(t *testing.T) {
	app, _ := tv7ScopeApp(t)
	filter := uistate.CurrentTxFilter()
	filter.From, filter.To = "2026-03-01", "2026-03-31"
	filter.Text = "Coffee"
	filter.PeriodPreset = txnfilter.PeriodLastMonth
	// A named period with no matches still offers widening to all dates.
	uistate.PersistTxFilter(filter)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnToolbarWidget, txnToolbarProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}}))
	if byTestID(f, "button", "txn-search-all-dates") == nil {
		t.Fatalf("zero results hid recovery: %s", f.Text())
	}
	found := false
	for _, n := range f.AllByTag("span") {
		if n.Attr("role") == "status" && strings.Contains(n.Text(), "0 transactions") {
			found = true
		}
	}
	if !found {
		t.Fatal("zero result status disappeared")
	}
}

func TestCSTOOLW2FactsHaveOneVisibleSource(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	shown := txnV5RowTransfers(rows, false, currency.Rates{Base: "USD"})
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Fragment(ui.CreateElement(txnToolbarWidget, txnToolbarProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}, Shown: shown}), txnTableScopedCountFacts(app, uistate.CurrentTxFilter(), rows, "USD", nil)))
	if byTestID(f, "div", "txn-table-count-facts") == nil {
		t.Fatal("table facts missing")
	}
	if facts := byTestID(f, "div", "txn-count-visible-facts"); facts != nil && strings.Contains(facts.Text(), "transactions") {
		t.Fatalf("duplicated transaction facts: %s", facts.Text())
	}
}

func TestCSTOOLW2MultiCategoryMatchesLegendScope(t *testing.T) {
	app, _ := tv7ScopeApp(t)
	filter := uistate.CurrentTxFilter()
	filter.Categories = "food"
	filter.ScopeAny = true
	uistate.PersistTxFilter(filter)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(txnCountLine(txnCountLineProps{Shown: 1, Total: 4}))
	if byTestID(f, "button", "txn-count-income") != nil || byTestID(f, "div", "txn-count-net") != nil {
		t.Fatal("multi-category filter presented partial income and net as household cashflow")
	}
	categories := txnCountCategories(filter, nil)
	if len(categories) != 2 {
		t.Fatalf("multi-category breakdown lost context: %+v (app %p)", categories, app)
	}
}

func TestCSTOOLW2NaturalLanguageCallerUsesLocalToday(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	previousLocation := time.Local
	offset := 14 * 3600
	if time.Now().UTC().Hour() < 12 {
		offset = -12 * 3600
	}
	time.Local = time.FixedZone("household", offset)
	defer func() { time.Local = previousLocation }()
	localDay := time.Now().Format("2006-01-02")
	if localDay == time.Now().UTC().Format("2006-01-02") {
		t.Fatal("fixture did not cross the UTC day")
	}
	filter := uistate.CurrentTxFilter()
	filter.Text = "today"
	uistate.PersistTxFilter(filter)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnToolbarWidget, txnToolbarProps{App: app, Base: "USD", Rates: currency.Rates{Base: "USD"}, Shown: rows}))
	apply := byTestID(f, "button", "nl-interpret")
	if apply == nil {
		t.Fatalf("interpret action missing: %s", f.Text())
	}
	apply.Click()
	if got := uistate.CurrentTxFilter().From; got != localDay {
		t.Fatalf("today applied %q, local wall-clock day is %q", got, localDay)
	}
}

func TestCSTOOLW2ReusesScopedHostDerivation(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	rates := currency.Rates{Base: "USD"}
	value, err := txntoolbar.Derive(rows, uistate.CurrentTxFilter(), txnfilter.Labels{}, rates, "USD", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// A sentinel identifies the supplied host result. Recalculating from rows
	// silently replaces it. Native tests separately prove the fold's arithmetic.
	value.View.Spent = 12345
	snapshot := &txntoolbar.Snapshot{Revision: app.Rev(), Scope: txnDerivedScope(app, uistate.CurrentTxFilter()), Value: value}
	props := txnToolbarProps{App: app, Base: "USD", Rates: rates, Shown: rows}
	// Reflection keeps this regression runnable with a source overlay of the old
	// props shape, so its red phase fails on behaviour rather than compilation.
	field := reflect.ValueOf(&props).Elem().FieldByName("Derived")
	if field.IsValid() {
		field.Set(reflect.ValueOf(snapshot))
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnToolbarWidget, props))
	if spent := byTestID(f, "span", "txn-count-spent-value"); spent == nil || spent.Text() != "$123.45" {
		t.Fatalf("toolbar ignored the supplied host derivation: %s", f.Text())
	}
}
