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

func TestR1ACountLineGroupsTransactionCounts(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 1250, Total: 3292, Net: money.New(42500, "USD"),
		Spent: money.New(10000, "USD"), Income: money.New(52500, "USD"), CashflowProvided: true}))
	got := f.Text()
	for _, want := range []string{"October 2026", "1,250 transactions", "Spent", "$100.00", "Income", "$525.00", "Net", "+$425.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("count line = %q, want current summary format containing %q", got, want)
		}
	}
	spent := renderedButton(f, "txn-count-spent")
	if spent == nil || !strings.Contains(spent.Attr("class"), "text-down") {
		t.Fatalf("spent count-line class = %v, want negative tone", spent)
	}
	if strings.Contains(got, "of 3,292") {
		t.Fatalf("count line = %q, want R2A format without an all-ledger denominator", got)
	}
}

func setupTxnCountCashflowFixture(t *testing.T) {
	t.Helper()
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("create count-line fixture app: %v", err)
	}
	previous := appstate.Default
	previousFilter := uistate.CurrentTxFilter()
	appstate.Default = app
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("set fixture base currency: %v", err)
	}
	uistate.PersistTxFilter(txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31", PeriodPreset: txnfilter.PeriodThisMonth})
	t.Cleanup(func() {
		appstate.Default = previous
		uistate.PersistTxFilter(previousFilter)
	})
	date := time.Date(2026, time.October, 15, 0, 0, 0, 0, time.UTC)
	for _, txn := range []domain.Transaction{
		{ID: "countline-expense", AccountID: "checking", Date: date, Desc: "Coffee groceries", Amount: money.New(-10000, "USD")},
		{ID: "countline-income", AccountID: "checking", Date: date, Desc: "Coffee pay", Amount: money.New(52500, "USD")},
	} {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatalf("put count-line fixture transaction: %v", err)
		}
	}
}

func TestR1ACountLineAnnouncesFilterChangesButNotPageOrSort(t *testing.T) {
	base := txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31", Page: 1, PageSize: 25, Sort: "date", Dir: "desc"}
	page := base
	page.Page = 2
	page.Sort, page.Dir = "amount", "asc"
	if txnAnnouncementScope(base) != txnAnnouncementScope(page) {
		t.Fatal("page or sort change altered the scope that drives the count-line live region")
	}
	changed := page
	changed.Category = "food"
	if txnAnnouncementScope(base) == txnAnnouncementScope(changed) {
		t.Fatal("filter change did not alter the count-line announcement scope")
	}
	if got := liveMode(false); got != "off" {
		t.Fatalf("steady-state aria-live = %q, want off", got)
	}
	if got := liveMode(true); got != "polite" {
		t.Fatalf("filter-change aria-live = %q, want polite", got)
	}
}
