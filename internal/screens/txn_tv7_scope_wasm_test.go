// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/debounce"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetsource"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func tv7ScopeApp(t *testing.T) (*appstate.App, []domain.Transaction) {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	ds := store.SampleDataset()
	ds.Members = []domain.Member{{ID: "marcus", Name: "Marcus Hartley"}, {ID: "priya", Name: "Priya Shah"}}
	ds.Accounts = []domain.Account{
		{ID: "checking", Name: "Joint Checking", Currency: "USD"},
		{ID: "loan", Name: "Car loan", Currency: "USD"},
	}
	ds.Categories = []domain.Category{{ID: "dining", Name: "Dining", Kind: domain.KindExpense}, {ID: "food", Name: "Groceries", Kind: domain.KindExpense}}
	date := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	ds.Transactions = []domain.Transaction{
		{ID: "coffee", AccountID: "checking", Date: date, Desc: "Coffee", Amount: money.New(-675, "USD"), MemberIDs: []string{"marcus"}},
		{ID: "groceries", AccountID: "checking", Date: date, Desc: "Market", CategoryID: "food", Amount: money.New(-21400, "USD"), Reviewed: true, MemberIDs: []string{"priya"}},
		{ID: "transfer-out", AccountID: "checking", TransferAccountID: "loan", Date: date, Desc: "Car payment", Amount: money.New(-48000, "USD"), Reviewed: true, MemberIDs: []string{"priya"}},
		{ID: "transfer-in", AccountID: "loan", TransferAccountID: "checking", Date: date, Desc: "Car payment", Amount: money.New(48000, "USD"), Reviewed: true, MemberIDs: []string{"priya"}},
	}
	// Keep both legacy legs as input: the ledger folds them, while the raw table fixture renders its supplied rows.
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	previous, previousFilter := appstate.Default, uistate.CurrentTxFilter()
	appstate.Default = app
	uistate.PersistTxFilter(txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31", PageSize: 25})
	t.Cleanup(func() {
		appstate.Default = previous
		uistate.PersistTxFilter(previousFilter)
		app.Store().Close()
	})
	return app, ds.Transactions
}

func tv7Nodes(root *render.QueryNode, match func(*render.QueryNode) bool) []*render.QueryNode {
	var result []*render.QueryNode
	var visit func(*render.QueryNode)
	visit = func(node *render.QueryNode) {
		if node == nil {
			return
		}
		if match(node) {
			result = append(result, node)
		}
		for _, child := range node.Children() {
			visit(child)
		}
	}
	visit(root)
	return result
}

func tv7AssertRows(t *testing.T, f *render.Fixture, rows []domain.Transaction, folded bool) {
	t.Helper()
	for _, txn := range rows {
		if folded && txn.ID == "transfer-in" {
			if byTestID(f, "tr", "txn-row-transfer-in") != nil {
				t.Error("the restored ledger duplicates the receiving leg of the legacy transfer")
			}
			continue
		}
		row := byTestID(f, "tr", "txn-row-"+txn.ID)
		if row == nil {
			t.Errorf("missing restored row %s", txn.ID)
			continue
		}
		pills := tv7Nodes(row, func(n *render.QueryNode) bool { return n.Attr("data-testid") == "txn-review-pill" })
		wantReview := txn.ID == "coffee"
		if (len(pills) == 1) != wantReview || len(pills) > 1 {
			t.Errorf("%s Review pills = %d, want review=%v", txn.ID, len(pills), wantReview)
		}
		category := "Uncategorized"
		if txn.CategoryID == "food" {
			category = "Groceries"
		}
		if txn.IsTransfer() {
			category = "Joint Checking → Car loan"

		}
		cells := tv7Nodes(row, func(n *render.QueryNode) bool { return n.Tag() == "td" && hasClass(n.Attr("class"), "td-cat") })
		if len(cells) != 1 || !strings.Contains(cells[0].Text(), category) {
			t.Errorf("%s category does not contain %q: %s", txn.ID, category, row.Text())
		}
		who := "Priya Shah"
		if txn.ID == "coffee" {
			who = "Marcus Hartley"
		}
		avatars := tv7Nodes(row, func(n *render.QueryNode) bool { return n.Attr("data-testid") == "txn-who-avatar" })
		if len(avatars) != 1 || avatars[0].Attr("title") != who {
			t.Errorf("%s Who must be %s: %s", txn.ID, who, row.Text())
		}
	}
}

func TestTV7SearchClearRestoresEveryRow(t *testing.T) {
	_, rows := tv7ScopeApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	defer func() { f.Render(Div()) }()
	defer debounce.Flush("txn-filter:text")
	f.Render(ui.CreateElement(Transactions))
	tv7AssertRows(t, f, rows, true)
	for _, query := range []string{"coffee", "", "zzqx", "coffee", ""} {
		search := byTestID(f, "input", "filter-toolbar-search")
		if search == nil {
			t.Fatal("mounted toolbar search is missing")
		}
		search.Input(query)
		// Timers only enqueue renders in this fixture. Wait for the real search
		// callback, then drain renders together on the test goroutine.
		deadline := time.Now().Add(5 * time.Second)
		for debounce.Pending("txn-filter:text") || uistate.CurrentTxFilter().Text != query {
			if time.Now().After(deadline) {
				t.Fatalf("search %q did not apply", query)
			}
			time.Sleep(5 * time.Millisecond)
		}
		f.Stabilize()
		if got := uistate.CurrentTxFilter().Text; got != query {
			t.Fatalf("applied query = %q, want %q", got, query)
		}
		if query == "" {
			tv7AssertRows(t, f, rows, true)
			if value := byTestID(f, "span", "txn-count-spent-value"); value == nil || value.Text() != "$220.75" {
				t.Errorf("cleared search did not restore the period's Spent figure: %v", value)
			}
			if value := byTestID(f, "span", "txn-count-moved"); value == nil || !strings.Contains(value.Text(), "$480.00") {
				t.Errorf("cleared search did not restore the period's transfer facts: %v", value)
			}
		} else if query == "coffee" {
			if byTestID(f, "tr", "txn-row-coffee") == nil || byTestID(f, "tr", "txn-row-groceries") != nil {
				t.Fatal("coffee search did not narrow the table")
			}
		} else if byTestID(f, "tr", "txn-row-coffee") != nil {
			t.Fatal("unmatched search retained a row")
		}
	}
}

func TestTV7CountFactsNeverUsesAnotherSearchView(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	previous := txnDerivedViewCache
	defer func() { txnDerivedViewCache = previous }()
	search := uistate.CurrentTxFilter()
	search.Text = "coffee"
	if _, err := txnDerivedViewCache.Get(app.Rev(), txnDerivedScope(app, search), rows[:1], currency.Rates{Base: "USD"}, "USD"); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(txnTableScopedCountFacts(app, uistate.CurrentTxFilter(), rows, "USD", nil))
	if value := byTestID(f, "span", "txn-count-moved"); value == nil || !strings.Contains(value.Text(), "$480.00") {
		t.Fatalf("table facts reused the coffee-only view: %s", f.Text())
	}
	// A helper with only row inputs must also derive from those rows, rather
	// than reading whichever App happens to be the process default.
	f.Render(txnTableCountFacts(rows, "USD", nil))
	if value := byTestID(f, "span", "txn-count-moved"); value == nil || !strings.Contains(value.Text(), "$480.00") {
		t.Fatal("row-only facts reused the active App's unrelated cached scope")
	}
}

func TestTV7RowEnrichmentFollowsSuppliedRowsAtSameRevisionAndFilter(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	f := render.New(t)
	defer f.Cleanup()
	defer func() { f.Render(Div()) }()
	renderRows := func(shown []domain.Transaction) {
		f.Rerender(ui.CreateElement(txnTableWidget, txnTableProps{
			Frame: widgetsource.RichTransactions(shown, app.Accounts(), app.Categories()),
			App:   app, Base: "USD", Shown: shown,
		}))
	}
	// Model the child notification before the parent supplies the cleared-search
	// rows. Both renders already see the same cleared filter and store revision.
	renderRows(rows[:1])
	renderRows(rows)
	tv7AssertRows(t, f, rows, false)
	// A count alone is insufficient: replacing one matching row with another
	// must also replace enrichment. Then restore the entire period again.
	renderRows(rows[:1])
	renderRows(rows[1:2])
	tv7AssertRows(t, f, rows[1:2], false)
	// The header's order cache must follow equal-length result replacements too.
	byTestID(f, "input", "txn-select-all-visible").Click()
	if row := byTestID(f, "tr", "txn-row-groceries"); row == nil || !hasClass(row.Attr("class"), "selected") {
		t.Error("select-all used an earlier frame's row identities")
	}
	byTestID(f, "input", "txn-select-all-visible").Click()
	renderRows(rows)
	tv7AssertRows(t, f, rows, false)
}
