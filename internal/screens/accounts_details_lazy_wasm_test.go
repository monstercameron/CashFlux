// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func clearAccountDetailCaches() {
	accountHistoryCache.Clear()
	accountSeriesCache.Clear()
	accountDetailFactsCache.Clear()
	accountProjectionInputsCache.Clear()
}

func TestAccountsHostDefersDetailsAndReusesSelectedHistory(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	filters := state.NewGlobalAtom("accounts:filter", uistate.AccountsFilter{})
	previousFilter := filters.Get()
	filters.Set(uistate.AccountsFilter{})
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousLens := lens.Get()
	lens.Set(scope.ReportScope{})
	t.Cleanup(func() { filters.Set(previousFilter); lens.Set(previousLens); clearAccountDetailCaches() })
	clearAccountDetailCaches()
	now := time.Now()
	checking := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD"), BalanceAsOf: now}
	locked := checking
	locked.ID, locked.Name, locked.Type, locked.OpeningBalance, locked.LockUntil = "locked", "Locked", domain.TypeSavings, money.New(5000, "USD"), now.AddDate(0, 1, 0)
	for _, account := range []domain.Account{checking, locked} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	for _, txn := range []domain.Transaction{
		{ID: "old", AccountID: checking.ID, Date: now.AddDate(0, 0, -500), Amount: money.New(1000, "USD"), Cleared: true},
		{ID: "recent", AccountID: checking.ID, Date: now.AddDate(0, 0, -10), Amount: money.New(-2000, "USD"), Cleared: true},
		{ID: "excluded", AccountID: checking.ID, Date: now.AddDate(0, 0, -2), Amount: money.New(100, "USD"), ExcludeFromReports: true},
	} {
		if err := app.Store().PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.Store().PutRecurring(domain.Recurring{ID: "rent", Label: "Rent", AccountID: checking.ID,
		Amount: money.New(-3000, "USD"), NextDue: now.AddDate(0, 0, 1), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutRecurring(domain.Recurring{ID: "late-reserve", Label: "Later reserve", AccountID: checking.ID,
		Amount: money.New(-50000, "USD"), NextDue: now.AddDate(0, 0, 60), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()
	if byTestID(f, "button", "acct-details-toggle-checking") == nil || byTestID(f, "button", "acct-details-toggle-locked") == nil {
		t.Fatalf("actual account host did not render its rows: %s", f.Text())
	}
	if accountHistoryCache.Len() != 0 || accountSeriesCache.Len() != 0 || accountDetailFactsCache.Len() != 0 || accountProjectionInputsCache.Len() != 0 {
		t.Fatal("compact host eagerly prepared account detail facts")
	}
	if !strings.Contains(byTestID(f, "button", "acct-balance-btn-checking").Text(), "91.00") || byTestID(f, "div", "acct-details-checking") != nil {
		t.Fatalf("compact exact balance or disclosure changed: %s", f.Text())
	}
	var order []string
	for _, row := range f.AllByTag("div") {
		if id := row.Attr("data-testid"); strings.HasPrefix(id, "acct-row-") {
			order = append(order, id)
		}
	}
	if !reflect.DeepEqual(order, []string{"acct-row-checking", "acct-row-locked"}) {
		t.Fatalf("account sorting changed: %v", order)
	}
	byTestID(f, "button", "acct-details-toggle-checking").Click()
	f.Stabilize()
	if accountHistoryCache.Len() != 1 || accountSeriesCache.Len() != 1 || accountDetailFactsCache.Len() != 1 || accountProjectionInputsCache.Len() != 1 {
		t.Fatal("one disclosure did not prepare exactly one selected account/window")
	}
	chart := byTestID(f, "svg", "acct-spark-checking")
	want := accountflow.Polyline(accountflow.BalanceSeries(checking, app.TransactionsByAccount(checking.ID), now, 90), sparklineW, sparklineH, 2)
	if chart == nil || len(chart.Children()) != 1 || chart.Children()[0].Attr("points") != want || byTestID(f, "div", "acct-proj-checking") == nil {
		t.Fatal("expanded row's history/projection differs from the previous display")
	}
	if byTestID(f, "span", "acct-fwd-negative-checking") == nil {
		t.Fatalf("90-day forward line missed the recurring debit due after day 30: %s", f.Text())
	}
	byTestID(f, "button", "acct-range-all-checking").Click()
	f.Stabilize()
	if accountHistoryCache.Len() != 1 || accountSeriesCache.Len() != 2 || accountDetailFactsCache.Len() != 1 {
		t.Fatal("range switch rebuilt history/projection or prepared unselected windows")
	}
	for i := 0; i < 2; i++ {
		byTestID(f, "button", "acct-details-toggle-checking").Click()
		f.Stabilize()
	}
	if accountSeriesCache.Len() != 2 || accountDetailFactsCache.Len() != 1 || byTestID(f, "button", "acct-range-all-checking").Attr("aria-checked") != "true" {
		t.Fatalf("collapse/reopen discarded the selected range or repeated its preparation: series=%d facts=%d selected=%s", accountSeriesCache.Len(), accountDetailFactsCache.Len(), byTestID(f, "button", "acct-range-all-checking").Attr("aria-checked"))
	}
	byTestID(f, "button", "acct-proj-toggle-checking").Click()
	f.Stabilize()
	if drivers := byTestID(f, "div", "acct-proj-drivers-checking"); drivers == nil || !strings.Contains(drivers.Text(), "Rent") || strings.Contains(drivers.Text(), "Later reserve") {
		t.Fatal("30-day projection explanation lost its near-term driver or included a later flow")
	}
	if err := app.Store().PutTransaction(domain.Transaction{ID: "new", AccountID: checking.ID, Date: now, Amount: money.New(-1000, "USD")}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(byTestID(f, "button", "acct-balance-btn-checking").Text(), "81.00") || byTestID(f, "div", "acct-details-checking") == nil || byTestID(f, "button", "acct-range-all-checking").Attr("aria-checked") != "true" {
		t.Fatalf("ledger revision lost disclosure state or kept an obsolete balance: %s", f.Text())
	}
	if accountHistoryCache.Len() != 2 || accountSeriesCache.Len() != 3 || accountDetailFactsCache.Len() != 2 {
		t.Fatal("ledger revision retained stale detail facts or eagerly prepared compact siblings")
	}
	// The existing balance action still opens the correct shell editor.
	editor := state.NewGlobalAtom("accounts:edit", uistate.AccountEdit{})
	previousEditor := editor.Get()
	t.Cleanup(func() { editor.Set(previousEditor) })
	byTestID(f, "button", "acct-balance-btn-checking").Click()
	f.Stabilize()
	if got := editor.Get(); got.ID != checking.ID || got.Mode != uistate.AcctEditModeSetBal {
		t.Fatalf("balance action changed: %+v", got)
	}
}

func TestAccountDetailCachesIncludeAppRatesAndExactTime(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	clearAccountDetailCaches()
	t.Cleanup(clearAccountDetailCaches)
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	account := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1000, "USD")}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutRecurring(domain.Recurring{ID: "intraday", Label: "Later today", AccountID: account.ID,
		Amount: money.New(-100, "USD"), NextDue: now.Add(time.Hour), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	rates := currency.Rates{Base: "USD"}
	context := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now)
	first := context.facts(account)
	if context.facts(account) != first || len(first.Projection.Drivers) != 1 {
		t.Fatal("identical context did not reuse the prepared projection")
	}
	later := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now.Add(2*time.Hour)).facts(account)
	if later == first || len(later.Projection.Drivers) != 0 {
		t.Fatal("exact occurrence time retained a projection from earlier in the day")
	}
	changedRates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	if changed := newAccountDetailContext(app, accountSnapshot(app), changedRates, "USD", now).facts(account); changed == first {
		t.Fatal("rate/presentation context shared another facts identity")
	}
	series, _ := context.series(account, "90d")
	otherZone := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.FixedZone("west", -5*60*60))
	zoned, _ := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", otherZone).series(account, "90d")
	if &series[0] == &zoned[0] || !reflect.DeepEqual(series, zoned) || accountHistoryCache.Len() != 1 {
		t.Fatal("same-date timezone contexts shared a series identity or rebuilt the history index")
	}
	nextDay, _ := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now.AddDate(0, 0, 1)).series(account, "90d")
	if &series[0] == &nextDay[0] || accountHistoryCache.Len() != 1 {
		t.Fatal("day change retained the series or unnecessarily rebuilt its history index")
	}
	other := primaryAccountsGoalsApp(t)
	account.OpeningBalance.Amount = 2000
	if err := other.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	// The global revision can be identical across these apps. Instance identity
	// must still separate balances, history and projection facts.
	left := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now)
	right := newAccountDetailContext(other, accountSnapshot(other), rates, "USD", now)
	if left.facts(app.Accounts()[0]).Projection.Start != 1000 || right.facts(account).Projection.Start != 2000 {
		t.Fatal("account details leaked across app instances")
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 3}}); err != nil {
		t.Fatal(err)
	}
	if newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now).facts(app.Accounts()[0]) == left.facts(app.Accounts()[0]) {
		t.Fatal("store revision retained old account facts")
	}
}

func TestAccountsHostRejectsHistoryAndProjectionForInvalidLedgerBalance(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	filters := state.NewGlobalAtom("accounts:filter", uistate.AccountsFilter{})
	previous := filters.Get()
	filters.Set(uistate.AccountsFilter{})
	clearAccountDetailCaches()
	t.Cleanup(func() { filters.Set(previous); clearAccountDetailCaches() })
	account := domain.Account{ID: "invalid", Name: "Invalid", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1000, "USD")}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutTransaction(domain.Transaction{ID: "foreign", AccountID: account.ID, Date: time.Now(), Amount: money.New(100, "EUR")}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutRecurring(domain.Recurring{ID: "bill", Label: "Bill", AccountID: account.ID,
		Amount: money.New(-100, "USD"), NextDue: time.Now().AddDate(0, 0, 1), Cadence: domain.CadenceMonthly}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()
	byTestID(f, "button", "acct-details-toggle-invalid").Click()
	f.Stabilize()
	if byTestID(f, "p", "acct-history-unavailable-invalid") == nil || byTestID(f, "svg", "acct-spark-invalid") != nil || byTestID(f, "p", "acct-proj-unavailable-invalid") == nil || byTestID(f, "p", "acct-fwd-unavailable-invalid") == nil {
		t.Fatalf("invalid exact balance became a valid chart or plausible-zero projection: %s", f.Text())
	}
	if accountHistoryCache.Len() != 0 || accountSeriesCache.Len() != 0 || accountProjectionInputsCache.Len() != 0 {
		t.Fatal("invalid ledger still prepared misleading financial curves")
	}
}

func TestAccountsHostShowsUnavailableForProjectionOverflowAcrossConsumers(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	filters := state.NewGlobalAtom("accounts:filter", uistate.AccountsFilter{})
	previous := filters.Get()
	filters.Set(uistate.AccountsFilter{})
	clearAccountDetailCaches()
	t.Cleanup(func() { filters.Set(previous); clearAccountDetailCaches() })
	account := domain.Account{ID: "overflow", Name: "Overflow", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(int64(^uint64(0)>>1), "USD")}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutRecurring(domain.Recurring{ID: "one-more", Label: "One more dollar", AccountID: account.ID,
		Amount: money.New(1, "USD"), NextDue: time.Now().AddDate(0, 0, 1), Cadence: domain.CadenceYearly}); err != nil {
		t.Fatal(err)
	}
	accounts := render.New(t, render.WithQueuedScheduler())
	defer accounts.Cleanup()
	accounts.Render(ui.CreateElement(Accounts))
	accounts.Stabilize()
	byTestID(accounts, "button", "acct-details-toggle-overflow").Click()
	accounts.Stabilize()
	if byTestID(accounts, "p", "acct-proj-unavailable-overflow") == nil || byTestID(accounts, "p", "acct-fwd-unavailable-overflow") == nil {
		t.Fatalf("projection overflow appeared as a healthy or plausible amount: %s", accounts.Text())
	}

	accounts.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: account.ID, Mode: uistate.AcctEditModeSetBal}))
	accounts.Stabilize()
	projected := byTestID(accounts, "span", "acct-bal-projected")
	if projected == nil || projected.Text() != uistate.T("calculation.unavailable") {
		t.Fatalf("editor hid projection arithmetic failure: %s", accounts.Text())
	}
}
