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
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestUXT1SummaryHasThreeLabelledFiguresAndVisibleSecondaryFacts(t *testing.T) {
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
	if r2aByTestID(f, "div", "txn-count-visible-facts") == nil {
		t.Fatal("countline must keep transfer, excluded, and conversion facts visible")
	}
	if !strings.Contains(f.Text(), "3 transactions") || !strings.Contains(f.Text(), "transferred between") {
		t.Fatalf("accessible summary omits transaction facts: %q", f.Text())
	}
	if !strings.Contains(f.Text(), "Net −$25.00") {
		t.Errorf("screen-reader summary must state the signed net once, got %q", f.Text())
	}
}

func TestCountLineTC3W13CategoryShareUsesBreakdownTotalWhenCategoryIsFiltered(t *testing.T) {
	rows := []txnCategoryTotal{{ID: "groceries", Minor: 20_000}, {ID: "rent", Minor: 150_000}}
	got := txnCategoryShare(rows[0].Minor, rows[0].Minor+rows[1].Minor)
	if got < 11.7 || got > 11.8 {
		t.Fatalf("selected category share = %.2f%%, want about 11.76%%", got)
	}
}

func TestCountLineTC3W13UncategorizedControlTogglesDedicatedFilter(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 3, Total: 3, Spent: money.New(12_500, "USD")}))
	item := findCategoryButton(f, "txn-category-item", "")
	if item == nil {
		t.Skip("fixture has no uncategorized expenses")
	}
	if item.Attr("aria-pressed") != "false" {
		t.Fatalf("initial uncategorized pressed state = %q", item.Attr("aria-pressed"))
	}
	item.Click()
	if !uistate.CurrentTxFilter().Uncategorized {
		t.Fatal("uncategorized click did not enable the dedicated filter")
	}
	item = findCategoryButton(f, "txn-category-item", "")
	if item == nil || item.Attr("aria-pressed") != "true" {
		t.Fatalf("uncategorized pressed state after click = %v", item)
	}
}

func TestUXT1CategoryLegendFiltersAndClearsCategory(t *testing.T) {
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
	if segment.Attr("aria-hidden") != "true" || segment.Attr("tabindex") != "-1" {
		t.Fatal("category bar segments must be decorative")
	}
	item.Click()
	if got := uistate.CurrentTxFilter().Category; got != "cat-food-uxt1" {
		t.Fatalf("category click filter = %q, want Food category", got)
	}
	findCategoryButton(f, "txn-category-item", "cat-food-uxt1").Click()
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
	if !strings.Contains(f.Text(), "0 posted. 0 scheduled. Due $0.00.") {
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
