// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func reviewAISetupLinkTestComponent(_ struct{}) ui.Node {
	return reviewAISetupLink(func() {})
}

func TestReviewTotalsOnlyNameNonBaseCurrencies(t *testing.T) {
	txns := []domain.Transaction{
		{Amount: money.New(-1250, "USD")},
		{Amount: money.New(-2500, "EUR")},
	}
	got := reviewCurrencyTotals(txns, "USD")
	if strings.Contains(got, "USD") {
		t.Fatalf("base currency code is redundant in review totals: %q", got)
	}
	if !strings.Contains(got, "EUR") || !strings.Contains(got, "$12.50") || !strings.Contains(got, "€25.00") {
		t.Fatalf("review totals should retain non-base currency context: %q", got)
	}
}

func TestReviewSmartSetupIsAOneLineFooterLink(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(reviewAISetupLinkTestComponent, struct{}{}))
	if got := f.Text(); got != "Set up Smart+ in Settings" {
		t.Fatalf("setup footer link text = %q", got)
	}
	var link *render.QueryNode
	for _, node := range f.AllByTag("a") {
		if node.Attr("data-testid") == "review-ai-setup" {
			link = node
			break
		}
	}
	if link == nil {
		t.Fatal("Smart+ setup footer link is missing")
	}
	if got := link.Attr("href"); got != "/settings/ai" {
		t.Fatalf("setup link destination = %q, want /settings/ai", got)
	}

	f.Render(ui.CreateElement(reviewScanStrip, reviewScanStripProps{HasProvider: false}))
	if got := strings.TrimSpace(f.Text()); got != "" {
		t.Fatalf("missing-key upsell still occupies the review body: %q", got)
	}
}

func TestReviewUnfilteredScopeDoesNotRepeatQueueSummary(t *testing.T) {
	f := render.New(t)
	f.Render(reviewScopeStrip(reviewScopeStripProps{
		Scope: reviewscope.ScopeEntireQueue, QueueTotal: 249,
	}))
	if got := strings.TrimSpace(f.Text()); got != "" {
		t.Fatalf("unfiltered scope repeats the queue summary: %q", got)
	}
}

func TestReviewBulkRowNamesNewCategoryAndOpenCharges(t *testing.T) {
	app := newReviewApp(t)
	txn := putCharge(t, app, "review-r1g", "ACME MARKET", 12)
	row := reviewRow{Group: reviewqueue.Group{
		Key: "acme-market", Merchant: "Acme Market", Items: []domain.Transaction{txn},
	}}
	f := render.New(t)
	f.Render(ui.CreateElement(reviewGroupRow, reviewGroupRowProps{
		Row: row, App: app, OnToggleOpen: func(string) {},
		OnToggleSelect: func(string) {}, OnCategory: func(string, string) {},
	}))

	newCategory := renderedButton(f, "review-newcat-acme-market")
	if newCategory == nil || newCategory.Attr("aria-label") != "New category" {
		t.Fatalf("plus control label = %v, want New category", newCategory)
	}
	openCharges := renderedButton(f, "review-expand-acme-market")
	if openCharges == nil || openCharges.Attr("aria-label") != "Open charges from Acme Market" {
		t.Fatalf("charge expander label = %v, want Open charges from Acme Market", openCharges)
	}
	if openCharges.Text() != "▸" {
		t.Fatalf("charge expander glyph = %q, want a chevron", openCharges.Text())
	}
}

func TestReviewMoneyUsesLedgerDisplay(t *testing.T) {
	for _, tc := range []struct {
		amount int64
		want   string
	}{{amount: -675, want: "−$6.75"}, {amount: 675, want: "+$6.75"}} {
		if got := reviewMoneyText(money.New(tc.amount, "USD")); got != tc.want {
			t.Errorf("reviewMoneyText(%d) = %q, want %q", tc.amount, got, tc.want)
		}
	}
	f := render.New(t)
	f.Render(reviewMoneyNode("review-expense", money.New(-675, "USD")))
	expense := f.AllByTag("span")[0]
	if expense.Text() != "−$6.75" || !strings.Contains(expense.Attr("class"), "text-down") {
		t.Fatalf("expense display text/class = %q / %q, want −$6.75 in negative tone", expense.Text(), expense.Attr("class"))
	}
	f.Render(reviewMoneyNode("review-amount", money.New(675, "USD")))
	node := f.AllByTag("span")[0]
	if node.Text() != "+$6.75" || !strings.Contains(node.Attr("class"), "text-up") {
		t.Fatalf("income display text/class = %q / %q, want +$6.75 in ledger-positive tone", node.Text(), node.Attr("class"))
	}
}

func renderedButton(f *render.Fixture, testID string) *render.QueryNode {
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == testID {
			return node
		}
	}
	return nil
}
