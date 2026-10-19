// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestUXT1SummaryHasThreeLabelledFiguresAndQuietSecondaryFacts(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{
		Shown: 3, Total: 20, Net: money.New(-2500, "USD"),
		Spent: money.New(12500, "USD"), Income: money.New(10000, "USD"),
		Moved: money.New(148000, "USD"), MovedRows: 1, Excluded: 2, Unconverted: 1,
	}))
	for _, id := range []string{"txn-count-spent", "txn-count-income", "txn-count-net"} {
		if r2aByTestID(f, "button", id) == nil && r2aByTestID(f, "div", id) == nil {
			t.Errorf("summary is missing figure %q", id)
		}
	}
	for _, label := range []string{"Spent", "Income", "Net"} {
		if !strings.Contains(f.Text(), label) {
			t.Errorf("summary is missing %q label: %q", label, f.Text())
		}
	}
	secondary := r2aByTestID(f, "div", "txn-count-secondary")
	if secondary == nil {
		t.Fatal("secondary facts line is missing")
	}
	if got := secondary.Text(); strings.ContainsAny(got, "·→") {
		t.Fatalf("secondary facts use decorative separators: %q", got)
	}
	for _, fact := range []string{"3 transactions", "transferred between", "left out of budgets", "not included"} {
		if !strings.Contains(secondary.Text(), fact) {
			t.Errorf("secondary facts missing %q: %q", fact, secondary.Text())
		}
	}
	if !strings.Contains(f.Text(), "Net −$25.00") {
		t.Errorf("screen-reader summary must state the signed net once, got %q", f.Text())
	}
}

func TestUXT1CategoryBarSegmentsFilterAndClearCategory(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	app := appstate.Default
	if err := app.PutCategory(domain.Category{ID: "cat-food-uxt1", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("put Food category: %v", err)
	}
	if err := app.Store().PutTransaction(domain.Transaction{
		ID: "uxt1-food-row", AccountID: "checking", Date: time.Date(2026, time.October, 16, 0, 0, 0, 0, time.UTC),
		Desc: "Groceries", CategoryID: "cat-food-uxt1", Amount: money.New(-2500, "USD"),
	}); err != nil {
		t.Fatalf("put Food transaction: %v", err)
	}
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 3, Total: 3, Spent: money.New(12500, "USD"), Income: money.New(52500, "USD"), Net: money.New(40000, "USD")}))
	segment := findCategoryButton(f, "txn-category-segment", "cat-food-uxt1")
	item := findCategoryButton(f, "txn-category-item", "cat-food-uxt1")
	if segment == nil || item == nil {
		t.Fatalf("Food category must be represented by a bar segment and equivalent list item: segment=%v item=%v", segment, item)
	}
	if !strings.Contains(f.Text(), "Food") {
		t.Fatalf("accessible category labels missing from summary: %q", f.Text())
	}
	segment.Click()
	if got := uistate.CurrentTxFilter().Category; got != "cat-food-uxt1" {
		t.Fatalf("category click filter = %q, want Food category", got)
	}
	findCategoryButton(f, "txn-category-segment", "cat-food-uxt1").Click()
	if got := uistate.CurrentTxFilter().Category; got != "" {
		t.Fatalf("second category click filter = %q, want cleared", got)
	}
}

func TestUXT1FuturePeriodKeepsScheduledAndDueFacts(t *testing.T) {
	previous := appstate.Default
	previousFilter := uistate.CurrentTxFilter()
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("create future summary app: %v", err)
	}
	appstate.Default = app
	uistate.PersistTxFilter(txnfilter.Criteria{From: "2099-10-01", To: "2099-10-31", PeriodPreset: txnfilter.PeriodCustom})
	t.Cleanup(func() {
		appstate.Default = previous
		uistate.PersistTxFilter(previousFilter)
	})
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Total: 0}))
	if !strings.Contains(f.Text(), "October 2099") || !strings.Contains(f.Text(), "0 scheduled · $0.00 due") {
		t.Fatalf("future-period summary lost its schedule and due facts: %q", f.Text())
	}
}

func findCategoryButton(f *render.Fixture, testID, categoryID string) *render.QueryNode {
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == testID && node.Attr("data-category-id") == categoryID {
			return node
		}
	}
	return nil
}
