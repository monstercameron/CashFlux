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
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestV5RowTransferShowsOnceByDefaultAndKeepsAccountLeg(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "move-1", Desc: "Move money", Amount: money.New(-5000, "USD")},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "move-1", Desc: "Move money", Amount: money.New(5000, "USD")},
		{ID: "coffee", AccountID: "checking", Desc: "Coffee", Amount: money.New(-500, "USD")},
	}

	visible := txnV5RowTransfers(rows, false)
	if len(visible) != 2 || visible[0].ID != "out" || visible[1].ID != "coffee" {
		t.Fatalf("unfiltered rows = %#v, want outgoing transfer once plus coffee", visible)
	}
	if got := len(visible); got != 2 {
		t.Fatalf("count/pager total = %d, want 2 visible transactions", got)
	}
	accountRows := txnV5RowTransfers(rows[1:2], true)
	if len(accountRows) != 1 || accountRows[0].ID != "in" {
		t.Fatalf("account-filtered leg = %#v, want destination leg", accountRows)
	}
}

func TestV5RowCountTotalDeduplicatesOnlyWithoutAccountFilter(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "move-1", Amount: money.New(-5000, "USD")},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "move-1", Amount: money.New(5000, "USD")},
		{ID: "coffee", AccountID: "checking", Amount: money.New(-500, "USD")},
	}
	if got := txnV5RowCountTotal(rows, "", ""); got != 2 {
		t.Fatalf("unfiltered total = %d, want 2 unique transactions", got)
	}
	if got := txnV5RowCountTotal(rows, "checking", ""); got != 3 {
		t.Fatalf("account-filtered total = %d, want all 3 ledger legs", got)
	}
}

func TestTC3W13V5RowKeepsEveryLegacyTransferLeg(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "legacy-out", AccountID: "checking", TransferAccountID: "savings", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(-50_000, "USD")},
		{ID: "legacy-in", AccountID: "savings", TransferAccountID: "checking", Date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Amount: money.New(20_000, "USD")},
	}
	got := txnV5RowTransfers(rows, false)
	if len(got) != 2 {
		t.Fatalf("legacy transfers after collapse = %v, want both rows retained", got)
	}
}

func TestTC3W13BulkSelectionIncludesExpandedTransferLeg(t *testing.T) {
	selected := map[string]bool{"checking-leg": true}
	got := txnV5RowExpandedSelection(selected, []string{"checking-leg", "savings-leg"})
	if len(got) != 2 || !got["checking-leg"] || !got["savings-leg"] {
		t.Fatalf("expanded selection = %v, want both transfer legs", got)
	}
}

func TestTC3W13BulkSelectionUsesAppTransferExpansion(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	rows := []domain.Transaction{
		{ID: "checking-leg", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "bulk-pair", Amount: money.New(-1_000, "USD")},
		{ID: "savings-leg", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "bulk-pair", Amount: money.New(1_000, "USD")},
	}
	for _, row := range rows {
		if err := app.Store().PutTransaction(row); err != nil {
			t.Fatalf("store %s: %v", row.ID, err)
		}
	}
	got := txnExpandedSelection(app, map[string]bool{"checking-leg": true})
	if !got["checking-leg"] || !got["savings-leg"] || len(got) != 2 {
		t.Fatalf("selection expanded to %v, want both legs", got)
	}
}

func TestV5RowTableCountFactsRenderBesideTable(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "move-1", Amount: money.New(-5000, "USD")},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "move-1", Amount: money.New(5000, "USD")},
		{ID: "coffee", AccountID: "checking", ExcludeFromReports: true, Amount: money.New(-500, "USD")},
		{ID: "eur", AccountID: "checking", Amount: money.New(-100, "EUR")},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(txnTableCountFacts(txnV5RowTransfers(rows, false), "USD", nil))
	if byTestID(f, "div", "txn-table-count-facts") == nil {
		t.Fatal("table count facts did not render beside the table")
	}
	if got := f.Text(); !strings.Contains(got, "3 transactions") || !strings.Contains(got, "$50.00 transferred between your accounts") || !strings.Contains(got, "left out of budgets") || !strings.Contains(got, "not included") {
		t.Fatalf("table facts = %q, want row count, one $50 transfer, excluded and unconverted rows", got)
	}
}

func TestTransactionEditBalanceAccountCount(t *testing.T) {
	if got := txnEditBalanceAccountCount(domain.Transaction{AccountID: "checking"}, domain.Transaction{AccountID: "checking"}); got != 1 {
		t.Fatalf("same-account edit affects %d accounts, want 1", got)
	}
	if got := txnEditBalanceAccountCount(domain.Transaction{AccountID: "checking"}, domain.Transaction{AccountID: "savings"}); got != 2 {
		t.Fatalf("account move affects %d accounts, want 2", got)
	}
	transfer := domain.Transaction{AccountID: "checking", TransferAccountID: "savings"}
	if got := txnEditBalanceAccountCount(transfer, transfer); got != 2 {
		t.Fatalf("transfer edit affects %d accounts, want 2", got)
	}
}

func TestV5RowTransferLabelNamesDirection(t *testing.T) {
	if got := txnV5RowTransferLabel("Checking", "Savings"); got != "Checking → Savings" {
		t.Fatalf("transfer label = %q", got)
	}
}

func TestV5RowCategoryControlNameIncludesValueAndRow(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	props := txnCategoryChooseProps{Label: "Dining", Description: "Coffee at Joe's"}
	f.Render(ui.CreateElement(txnCategoryChoose, props))
	button := byTestID(f, "button", "txn-category-choose")
	if button == nil {
		t.Fatal("category control did not render")
	}
	if got := button.Attr("aria-label"); got != "Category: Dining, Coffee at Joe's" {
		t.Fatalf("category accessible name = %q", got)
	}
}

func TestV5RowNamedActionsRetainRowContext(t *testing.T) {
	name := txnV5RowControlName(uistate.T("v5row.category"), "Groceries", "Coffee at Joe's")
	if !strings.Contains(name, "Groceries") || !strings.Contains(name, "Coffee at Joe's") {
		t.Fatalf("control name omits its value or row: %q", name)
	}
}
