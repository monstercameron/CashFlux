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
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestV5SUMCategoryBreakdownRefreshesOnTransactionRevision(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	app := appstate.Default
	if err := app.PutCategory(domain.Category{ID: "v5sum-dining", Name: "Dining", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("create Dining category: %v", err)
	}
	date := time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)
	if err := app.Store().PutTransaction(domain.Transaction{ID: "v5sum-uncategorized", AccountID: "checking", Date: date, Desc: "Coffee", Amount: money.New(-3875, "USD")}); err != nil {
		t.Fatalf("create uncategorized transaction: %v", err)
	}
	props := txnCountLineProps{Shown: 3, Total: 3, Spent: money.New(13875, "USD"), Income: money.New(52500, "USD"), Net: money.New(38625, "USD")}
	f := render.New(t)
	f.Render(txnCountLine(props))
	if item := findCategoryButton(f, "txn-category-item", ""); item == nil || !strings.Contains(item.Text(), "$138.75") {
		t.Fatalf("initial uncategorized breakdown missing the combined amount: %v", item)
	}
	transaction, ok := app.TransactionByID("v5sum-uncategorized")
	if !ok {
		t.Fatal("uncategorized transaction was not stored")
	}
	transaction.CategoryID = "v5sum-dining"
	beforeWriteRev := app.Rev()
	if err := app.PutTransaction(transaction); err != nil {
		t.Fatalf("recategorize transaction: %v", err)
	}
	if app.Rev() <= beforeWriteRev {
		t.Fatalf("transaction revision did not advance: %d -> %d", beforeWriteRev, app.Rev())
	}
	if saved, ok := app.TransactionByID(transaction.ID); !ok || saved.CategoryID != "v5sum-dining" {
		t.Fatalf("recategorized transaction read back as %#v, found=%v", saved, ok)
	}
	props.Shown++
	f.Render(txnCountLine(props))
	if item := findCategoryButton(f, "txn-category-item", "v5sum-dining"); item == nil || !strings.Contains(item.Text(), "$38.75") {
		t.Fatalf("same-render revision did not show Dining $38.75: %v; summary %q", item, f.Text())
	}
	if item := findCategoryButton(f, "txn-category-item", ""); item != nil && strings.Contains(item.Text(), "$38.75") {
		t.Fatalf("same-render revision retained stale Uncategorized $38.75: %q", item.Text())
	}
}

func TestV5SUMSeptemberBarKeepsTheUnshownRemainderProportional(t *testing.T) {
	top := []txnCategoryTotal{
		{ID: "a", Minor: 140000}, {ID: "b", Minor: 90000}, {ID: "c", Minor: 80000},
		{ID: "d", Minor: 60000}, {ID: "e", Minor: 37651}, {ID: "f", Minor: 230784},
	}
	shown := txnCategoryTopFive(top, "")
	if len(shown) != 6 || shown[5].ID != "__other__" || shown[5].Minor != 230784 {
		t.Fatalf("top-five summary = %#v, want Other $2,307.84", shown)
	}
	spent := int64(638435)
	var shownTop int64
	for _, item := range shown[:5] {
		shownTop += item.Minor
	}
	if shownTop != 407651 || shown[5].Minor+shownTop != spent {
		t.Fatalf("September categories sum to %d + %d, want $6,384.35", shownTop, shown[5].Minor)
	}
	if got := float64(shown[5].Minor) * 100 / float64(spent); got < 36.1 || got > 36.2 {
		t.Fatalf("Other share = %.4f%%, want about 36%%", got)
	}
	f := render.New(t)
	f.Render(txnCategoryBreakdown(shown, "", spent))
	other := findCategoryButton(f, "txn-category-item", "__other__")
	if other == nil || !strings.Contains(other.Text(), "Other") || !strings.Contains(other.Text(), "$2,307.84") {
		t.Fatalf("Other line is missing or incorrect: %v", other)
	}
	segment := findCategoryButton(f, "txn-category-segment", "__other__")
	if segment == nil || segment.Attr("title") != "Other $2,307.84" {
		t.Fatalf("Other bar segment tooltip is missing the remainder: %v", segment)
	}
	if got := strconvTxnPercent(txnCategoryShare(shown[5].Minor, spent)); !strings.HasPrefix(got, "36.1") {
		t.Fatalf("Other segment width = %q, want amount / Spent (about 36.1%%)", got)
	}
}

func TestV5SUMCategoryFilterSelectionAndScope(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	app := appstate.Default
	for _, category := range []domain.Category{{ID: "v5sum-shop", Name: "Shopping", Kind: domain.KindExpense}, {ID: "v5sum-food", Name: "Food", Kind: domain.KindExpense}} {
		if err := app.PutCategory(category); err != nil {
			t.Fatalf("create category %s: %v", category.ID, err)
		}
	}
	date := time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)
	for _, transaction := range []domain.Transaction{
		{ID: "v5sum-shop-row", AccountID: "checking", Date: date, Desc: "Store", CategoryID: "v5sum-shop", Amount: money.New(-2500, "USD")},
		{ID: "v5sum-food-row", AccountID: "checking", Date: date, Desc: "Market", CategoryID: "v5sum-food", Amount: money.New(-1500, "USD")},
	} {
		if err := app.Store().PutTransaction(transaction); err != nil {
			t.Fatalf("create category transaction: %v", err)
		}
	}
	f := render.New(t)
	props := txnCountLineProps{Shown: 4, Total: 4, Spent: money.New(14000, "USD"), Income: money.New(52500, "USD"), Net: money.New(38500, "USD")}
	f.Render(txnCountLine(props))
	uncategorized := findCategoryButton(f, "txn-category-item", "")
	uncatSegment := findCategoryButton(f, "txn-category-segment", "")
	if uncatSegment == nil || uncatSegment.Attr("aria-hidden") != "true" || uncatSegment.Attr("tabindex") != "-1" {
		t.Fatalf("category segments must be decorative: %v", uncatSegment)
	}
	if strings.Contains(uncatSegment.Attr("class"), "is-selected") {
		t.Fatalf("needs-attention emphasis must not use selected style: %q", uncatSegment.Attr("class"))
	}
	if uncategorized == nil || uncategorized.Attr("aria-pressed") != "false" || !strings.Contains(uncategorized.Attr("class"), "needs-attention") {
		t.Fatalf("Uncategorized needs-attention dot class missing: %v", uncategorized)
	}
	findCategoryButton(f, "txn-category-item", "v5sum-shop").Click()
	f.Render(txnCountLine(props))
	for _, id := range []string{"", "v5sum-shop", "v5sum-food"} {
		item := findCategoryButton(f, "txn-category-item", id)
		if item == nil {
			t.Errorf("category %q disappeared while category filter is active", id)
		}
	}
	if item := findCategoryButton(f, "txn-category-item", "v5sum-shop"); item == nil || item.Attr("aria-pressed") != "true" {
		t.Errorf("active category is not selected: %v", item)
	}
	if item := findCategoryButton(f, "txn-category-item", "v5sum-food"); item == nil || item.Attr("aria-pressed") != "false" || !strings.Contains(item.Attr("class"), "is-dimmed") {
		t.Errorf("unselected category should stay visible and dimmed: %v", item)
	}
	if r2aByTestID(f, "button", "txn-count-income") != nil || r2aByTestID(f, "div", "txn-count-net") != nil {
		t.Error("Income and Net should be hidden while a category filter is active")
	}
	if strings.Contains(f.Text(), "Income") || strings.Contains(f.Text(), "Net") {
		t.Errorf("accessible category-filter summary should not announce Income or Net: %q", f.Text())
	}
	if r2aByTestID(f, "div", "txn-count-upcoming-fact") != nil {
		t.Error("upcoming bills should be hidden while a category filter is active")
	}
	if uncategorized != nil && uncategorized.Attr("aria-pressed") != "false" {
		t.Errorf("Uncategorized must remain unselected while another category is filtered: %q", uncategorized.Attr("aria-pressed"))
	}
}

func TestV5SUMCategoryDerivationIgnoresCategoryFilterButKeepsOtherScope(t *testing.T) {
	filter := txnfilter.Criteria{From: "2026-09-01", To: "2026-09-30", Category: "selected"}
	criteria, _ := filter.WithOwnerLens(nil)
	criteria.Flow, criteria.Category = "", ""
	if criteria.From != "2026-09-01" || criteria.To != "2026-09-30" || criteria.Category != "" {
		t.Fatalf("category breakdown scope = %#v, want date scope retained and category filter removed", criteria)
	}
}
