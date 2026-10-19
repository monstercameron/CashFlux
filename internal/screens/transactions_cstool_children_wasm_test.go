// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func cstoolRenderProps(t *testing.T) txnToolbarRenderProps {
	app, rows := tv7ScopeApp(t)
	if err := app.PutCustomFieldDef(customfields.Def{ID: "cstool-custom", EntityType: "transaction", Key: "project", Label: "Project", Type: customfields.TypeText}); err != nil {
		t.Fatal(err)
	}
	rates := currency.Rates{Base: "USD"}
	filter := uistate.CurrentTxFilter()
	filter.CustomKey = "project"
	derived, err := txntoolbar.Derive(rows, txnfilter.Criteria{}, txnfilter.Labels{}, rates, "USD", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return txnToolbarRenderProps{txnToolbarProps: txnToolbarProps{App: app, Base: "USD", Rates: rates, Shown: rows}, Own: filter, Filter: filter, Indexes: buildTxnToolbarIndexes(app.Accounts(), app.Categories(), app.Members(), rows), Derived: derived, Summary: txnToolbarSummary{review: 2, presets: derived.Presets}, SetFilter: func(func(*txnfilter.Criteria)) {}}
}

func TestCSTOOLToolbarChildComponents(t *testing.T) {
	for _, tt := range []struct {
		name, tag, id string
		child         func(txnToolbarRenderProps) ui.Node
	}{
		{"controls", "input", "filter-toolbar-search", txnToolbarControls},
		{"add", "button", "txn-add-btn", txnToolbarAdd},
		{"review", "button", "txn-review-btn", txnToolbarReview},
		{"more", "button", "txn-more-btn", txnToolbarMore},
		{"filters", "input", "filter-search-account", txnToolbarFilterPanel},
		{"summary", "span", "txn-count-spent-value", txnToolbarCount},
		{"interpret", "button", "nl-interpret", txnToolbarInterpret},
	} {
		t.Run(tt.name, func(t *testing.T) {
			props := cstoolRenderProps(t)
			if tt.name == "interpret" {
				props.Filter.Text = "today"
			}
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(tt.child, props))
			if byTestID(f, tt.tag, tt.id) == nil {
				t.Fatalf("%s missing %s: %s", tt.name, tt.id, f.Text())
			}
		})
	}
}

func TestCSTOOLCustomFilterChildOwnsTextInputHook(t *testing.T) {
	defer debounce.Flush("txn-filter:customval")
	props := cstoolRenderProps(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnToolbarCustomFilter, props))
	found := false
	for _, input := range f.AllByTag("input") {
		if input.Attr("aria-label") == uistate.T("transactions.filterCustomValue") {
			found = true
			input.Input("test")
		}
	}
	if !found {
		t.Fatal("custom filter text field missing")
	}
}

func TestCSTOOLBulkContentChildPreservesControls(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	props := txnBulkBarContentProps{Count: 2, OnCategory: func(string) {}, OnCreate: func(string) {}, OnOwners: func([]string) {}}
	f.Render(ui.CreateElement(txnBulkBarContent, props))
	for _, id := range []string{"bulk-category-picker", "bulk-owner-picker", "bulk-mark-reviewed", "bulk-delete", "bulk-clear-selection"} {
		if byTestID(f, "button", id) == nil {
			t.Fatalf("bulk content missing %s", id)
		}
	}
}

func TestCSTOOLHostDerivationSkipsToolbarLedgerPassesAndRejectsStaleProps(t *testing.T) {
	props := cstoolRenderProps(t)
	props.Own.CustomKey, props.Filter.CustomKey = "", ""
	value := props.Derived
	value.View.Spent = 12345
	value.Presets.Large = 777
	props.Derived = value
	props.txnToolbarProps.Derived = &txntoolbar.Snapshot{Revision: props.App.Rev(), Scope: txnDerivedScope(props.App, uistate.CurrentTxFilter()), Value: value}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnToolbarWidget, props.txnToolbarProps))
	if spent := byTestID(f, "span", "txn-count-spent-value"); spent == nil || spent.Text() != "$123.45" {
		t.Fatalf("host-derived view was recomputed: %s", f.Text())
	}
	// A host prop from a previous search is not authoritative for the new scope.
	stale := *props.txnToolbarProps.Derived
	stale.Scope = "old-search"
	props.txnToolbarProps.Derived = &stale
	f.Render(ui.CreateElement(txnToolbarWidget, props.txnToolbarProps))
	if spent := byTestID(f, "span", "txn-count-spent-value"); spent == nil || spent.Text() != "$220.75" {
		t.Fatalf("stale host snapshot reused: %s", f.Text())
	}
}
