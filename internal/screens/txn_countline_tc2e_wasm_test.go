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
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
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
	requireBandWithoutSecondaryFacts(t, f)
}

// requireBandWithoutSecondaryFacts fails unless the summary band (data-testid
// txn-count-band) is rendered and no element inside it carries the
// txn-count-secondary class; secondary facts belong beside the band.
func requireBandWithoutSecondaryFacts(t *testing.T, f *render.Fixture) {
	t.Helper()
	band := r2aByTestID(f, "div", "txn-count-band")
	if band == nil {
		t.Fatal("summary band txn-count-band is not rendered")
	}
	var visit func(*render.QueryNode) bool
	visit = func(node *render.QueryNode) bool {
		for _, child := range node.Children() {
			if hasClass(child.Attr("class"), "txn-count-secondary") || visit(child) {
				return true
			}
		}
		return false
	}
	if visit(band) {
		t.Fatal("summary band contains an element with class txn-count-secondary; secondary facts must render outside the band")
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
