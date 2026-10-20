// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
)

func seedSmartCategoryHistory(t *testing.T, c *documentsController) {
	t.Helper()
	if err := c.app.PutTransaction(domain.Transaction{
		ID: "history-merchant", AccountID: "acct-1", Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Desc: "merchant01", CategoryID: "cat-food", Amount: money.New(-125, "USD"),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestImportSmartUsesNinetyPercentHistoryMatch(t *testing.T) {
	f, current, _ := mountImportUX(t)
	seedSmartCategoryHistory(t, current())
	current().appendRows([]extract.Row{{Date: "2026-03-01", Description: "merchant02", Amount: "-1.00"}})
	f.Stabilize()
	findByTestID(f, "button", "suggest-categories-btn").Click()
	f.Stabilize()
	if got := current().model.Rows[0].Category; got != "Groceries" {
		t.Fatalf("Smart category = %q, want Groceries", got)
	}
}

func TestImportSmartMatchesHistoryAfterOnDeviceStatementParse(t *testing.T) {
	f, current, _ := mountImportUX(t)
	seedSmartCategoryHistory(t, current())
	current().importAcct.Set("")
	current().chooseSource("stmt")
	current().stmtText.Set("Date,Description,Amount\n2026-03-01,merchant02,-1.00")
	f.Stabilize()
	for _, form := range f.AllByTag("form") {
		if form.Attr("id") == "" {
			form.Submit()
			break
		}
	}
	f.Stabilize()
	if len(current().model.Rows) != 1 {
		t.Fatalf("local statement parsed %d rows", len(current().model.Rows))
	}
	findByTestID(f, "button", "suggest-categories-btn").Click()
	f.Stabilize()
	if got := current().model.Rows[0].Category; got != "Groceries" {
		t.Fatalf("Smart category = %q, row amount = %q, decimals = %d", got, current().model.Rows[0].Amount, current().recDec)
	}
}

func TestImportSmartDisplaysSampleMerchantCategory(t *testing.T) {
	f, current, _ := mountImportUX(t)
	if err := current().app.PutTransaction(domain.Transaction{
		ID: "history-greenfield", AccountID: "acct-1", Date: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC),
		Desc: "APLPAY GREENFIELD MKT #204", CategoryID: "cat-food", Amount: money.New(-2850, "USD"),
	}); err != nil {
		t.Fatal(err)
	}
	current().appendRows([]extract.Row{{Date: "2026-10-03", Description: "APLPAY GREENFIELD MKT #205", Amount: "-28.50"}})
	f.Stabilize()
	findByTestID(f, "button", "suggest-categories-btn").Click()
	f.Stabilize()
	if got := current().model.Rows[0].Category; got != "Groceries" {
		t.Fatalf("Smart category = %q, want Groceries", got)
	}
	if got := findByTestID(f, "div", "draft-review-row").Text(); !strings.Contains(got, "Groceries") {
		t.Fatalf("Smart category not visible in review row: %q", got)
	}
}

func TestImportSmartPlusSendsAggregatedHistoryToAI(t *testing.T) {
	calls := fakeImportFetch(t)
	f, current, dialog := mountImportUX(t)
	seedSmartCategoryHistory(t, current())
	current().appendRows([]extract.Row{{Date: "2026-03-01", Description: "merchant02", Amount: "-1.00"}})
	f.Stabilize()
	findByTestID(f, "button", "suggest-categories-ai").Click()
	if dialog() == nil || !strings.Contains(dialog().Message, "counts of closely matching past category choices") || calls.Length() != 0 {
		t.Fatal("Smart+ must disclose history before an AI request")
	}
	dialog().OnResult(true, "")
	if calls.Length() != 1 {
		t.Fatalf("Smart+ sent %d requests, want one", calls.Length())
	}
	body := calls.Index(0).Get("options").Get("body").String()
	if !strings.Contains(body, "matching_history") || !strings.Contains(body, "Groceries") || !strings.Contains(body, "merchant02") || strings.Contains(body, "merchant01") {
		t.Fatalf("AI request omitted compact history or leaked a past description: %s", body)
	}
	deliverImportContent(t, calls, 0, `{"items":[{"i":0,"category":"Groceries"}]}`)
	for deadline := time.Now().Add(time.Second); current().aiLoading.Get() && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		f.Stabilize()
	}
	f.Stabilize()
	if got := current().model.Rows[0].Category; got != "Groceries" {
		t.Fatalf("AI category = %q, want Groceries; error=%q loading=%v rows=%+v", got, current().aiErr.Get(), current().aiLoading.Get(), current().model.Rows)
	}
}
