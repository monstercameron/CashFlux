// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestTC2ECountLineReportsRowsThatCouldNotBeCompared(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	issues := []txnfilter.ConversionIssue{
		{Transaction: domain.Transaction{Amount: money.New(-12000, "EUR")}},
		{Transaction: domain.Transaction{Amount: money.New(-9000, "EUR")}},
	}
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{
		Shown: 1, Total: 2, Net: money.New(-12000, "USD"), Spent: money.New(12000, "USD"),
		ConversionIssues: issues,
	}))
	if got := f.Text(); !strings.Contains(got, "2 rows can't be compared (no EUR rate)") {
		t.Fatalf("count line = %q, want conversion issue summary", got)
	}
	if node := byTestID(f, "span", "txn-count-conversion-issues"); node == nil {
		t.Fatal("conversion issue summary is missing its test id")
	}
}

func TestTC2ESearchAllDatesUsesHouseholdCurrencyAndAppRevision(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("create JPY fixture app: %v", err)
	}
	previousApp := appstate.Default
	previousFilter := uistate.CurrentTxFilter()
	appstate.Default = app
	t.Cleanup(func() {
		appstate.Default = previousApp
		uistate.PersistTxFilter(previousFilter)
	})
	settings := app.Settings()
	settings.BaseCurrency = "JPY"
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("set JPY base currency: %v", err)
	}
	for i := 0; i < 12; i++ {
		txn := domain.Transaction{ID: "jpy-" + txnCountNumber(i), Desc: "Market", Amount: money.New(-1500, "JPY")}
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatalf("put JPY transaction %d: %v", i, err)
		}
	}
	filter := txnfilter.Criteria{Text: "1500"}
	if got := txnSearchAllDatesCount(filter); got != 12 {
		t.Fatalf("JPY all-dates search count = %d, want 12", got)
	}
	if err := app.Store().PutTransaction(domain.Transaction{ID: "jpy-new", Desc: "Market", Amount: money.New(-1500, "JPY")}); err != nil {
		t.Fatalf("put additional JPY transaction: %v", err)
	}
	if got := txnSearchAllDatesCount(filter); got != 13 {
		t.Fatalf("search count after app revision = %d, want 13", got)
	}
}
