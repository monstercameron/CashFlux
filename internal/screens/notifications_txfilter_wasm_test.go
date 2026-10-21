// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestNotificationTransactionActionPreservesLiveFilterOnLedgerMount(t *testing.T) {
	stubBudgetRowBrowser(t)
	doc := js.Global().Get("document")
	previousQuery := doc.Get("querySelectorAll")
	query := js.FuncOf(func(js.Value, []js.Value) any { return []any{} })
	doc.Set("querySelectorAll", query)
	defer func() { doc.Set("querySelectorAll", previousQuery); query.Release() }()
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := primaryTodoNotificationsApp(t)
	for _, id := range []string{"bank", "other-bank"} {
		if err := app.PutAccount(domain.Account{ID: id, Name: id, Class: domain.ClassAsset, Type: domain.TypeChecking,
			Currency: "USD", OpeningBalance: money.Zero("USD"), OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	day := dateutil.CalendarDayUTC(now)
	for _, txn := range []domain.Transaction{
		{ID: "lazy-target", AccountID: "bank", Payee: "Market", Desc: "Target market charge", Date: day, Amount: money.New(-1234, "USD")},
		{ID: "lazy-other-payee", AccountID: "bank", Payee: "Elsewhere", Desc: "Another merchant charge", Date: day, Amount: money.New(-2345, "USD")},
		{ID: "lazy-other-bank", AccountID: "other-bank", Payee: "Market", Desc: "Other bank charge", Date: day, Amount: money.New(-3456, "USD")},
		{ID: "lazy-other-date", AccountID: "bank", Payee: "Market", Desc: "Yesterday market charge", Date: day.AddDate(0, 0, -1), Amount: money.New(-4567, "USD")},
	} {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	stored := (uistate.TxFilter{Text: "Old saved search", PeriodPreset: txnfilter.PeriodCustom, From: "1999-01-01", To: "1999-01-31", Sort: "date", Dir: txnfilter.Desc}).Normalize()
	uistate.PersistTxFilter(stored)
	item := uistate.FeedItem{ID: "default-large@txn:lazy-target", Title: "Review target charge", Severity: "warning", At: now.Unix()}
	uistate.PersistNotifyFeed([]uistate.FeedItem{item})
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	var filter state.Atom[uistate.TxFilter]
	var lens state.Atom[scope.ReportScope]
	var mode state.Atom[string]
	var showLedger func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		filter, lens, mode = uistate.UseTxFilter(), uistate.UseActiveScope(), uistate.UseTxnViewMode()
		ledger := ui.UseState(false)
		showLedger = ledger.Set
		if ledger.Get() {
			return ui.CreateElement(Transactions)
		}
		return ui.CreateElement(NotificationCenter)
	}))
	lens.Set(scope.ReportScope{})
	mode.Set(uistate.TxnViewTable)
	// A debounced/live view can be newer than its stored seed. Every mounted row
	// must retain the shared atom rather than hydrate an old default on a rerender.
	latest := (uistate.TxFilter{Account: "bank", From: day.Format("2006-01-02"), To: day.Format("2006-01-02"), PeriodPreset: txnfilter.PeriodCustom, Sort: "amount", Dir: txnfilter.Asc}).Normalize()
	filter.Set(latest)
	f.Stabilize()
	primaryClick(t, f, "notif-ovf-"+item.ID)
	primaryClick(t, f, "notif-ovf-"+item.ID)
	if filter.Get() != latest || uistate.CurrentTxFilter() != stored {
		t.Fatal("notification menu rerender changed live or persisted filter state")
	}
	open := byTestID(f, "div", "notif-open-"+item.ID)
	if open == nil {
		t.Fatalf("actual notification host has no transaction action: %s", f.Text())
	}
	open.Click()
	f.Stabilize()
	want := latest
	want.Text = "Market"
	want = want.Normalize()
	if filter.Get() != want || uistate.CurrentTxFilter() != want {
		t.Fatalf("transaction action discarded latest account/date/sort: live=%+v stored=%+v want=%+v", filter.Get(), uistate.CurrentTxFilter(), want)
	}
	raw, ok := app.GetKV("cashflux:notify:feed")
	var feed []uistate.FeedItem
	if !ok || json.Unmarshal([]byte(raw), &feed) != nil || len(feed) != 1 || !feed[0].Read {
		t.Fatalf("opening transaction did not persist only its read flag: %s", raw)
	}
	// Mount the actual destination in the same runtime, as a client-side route
	// switch does. Its many late filter subscribers must observe the live action.
	showLedger(true)
	f.Stabilize()
	if byTestID(f, "tr", "txn-row-lazy-target") == nil {
		t.Fatalf("destination ledger lost transaction action filter: %s", f.Text())
	}
	for _, id := range []string{"lazy-other-payee", "lazy-other-bank", "lazy-other-date"} {
		if byTestID(f, "tr", "txn-row-"+id) != nil {
			t.Fatalf("destination ledger lost preserved filter dimension: %s", id)
		}
	}
	if filter.Get() != want {
		t.Fatalf("late destination subscribers reset the shared filter: %+v", filter.Get())
	}
}
