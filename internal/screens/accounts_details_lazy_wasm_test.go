// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/appstate"
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

// resetAccountsListFilters clears the accounts filter and household lens for one
// test and restores them afterwards.
func resetAccountsListFilters(t *testing.T) {
	t.Helper()
	filters := state.NewGlobalAtom("accounts:filter", uistate.AccountsFilter{})
	previousFilter := filters.Get()
	filters.Set(uistate.AccountsFilter{})
	lens := state.NewGlobalAtom("household:activeScope", scope.ReportScope{})
	previousLens := lens.Get()
	lens.Set(scope.ReportScope{})
	t.Cleanup(func() { filters.Set(previousFilter); lens.Set(previousLens) })
}

// lazyAccountsFixture is the Accounts screen over a shared checking account with an
// old, a recent and a report-excluded transaction, one near-term and one far-off
// recurring debit, and a locked savings account.
type lazyAccountsFixture struct {
	app      *appstate.App
	f        *render.Fixture
	checking domain.Account
	now      time.Time
}

func newLazyAccountsFixture(t *testing.T) *lazyAccountsFixture {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	resetAccountsListFilters(t)
	clearAccountDetailCaches()
	t.Cleanup(clearAccountDetailCaches)
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
	for _, rec := range []domain.Recurring{
		{ID: "rent", Label: "Rent", AccountID: checking.ID, Amount: money.New(-3000, "USD"), NextDue: now.AddDate(0, 0, 1), Cadence: domain.CadenceMonthly},
		{ID: "late-reserve", Label: "Later reserve", AccountID: checking.ID, Amount: money.New(-50000, "USD"), NextDue: now.AddDate(0, 0, 60), Cadence: domain.CadenceMonthly},
	} {
		if err := app.Store().PutRecurring(rec); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t, render.WithQueuedScheduler())
	t.Cleanup(f.Cleanup)
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()
	return &lazyAccountsFixture{app: app, f: f, checking: checking, now: now}
}

func (x *lazyAccountsFixture) click(t *testing.T, testID string) {
	t.Helper()
	button := byTestID(x.f, "button", testID)
	if button == nil {
		t.Fatalf("no button %q: %s", testID, x.f.Text())
	}
	button.Click()
	x.f.Stabilize()
}

// chartPoints returns the polyline of the checking account's balance chart, or ""
// when no chart is drawn.
func (x *lazyAccountsFixture) chartPoints() string {
	chart := byTestID(x.f, "svg", "acct-spark-checking")
	if chart == nil || len(chart.Children()) != 1 {
		return ""
	}
	return chart.Children()[0].Attr("points")
}

func TestAccountsHostDetailsDisclosure(t *testing.T) {
	t.Run("compact rows show the exact balance and no details", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		if byTestID(x.f, "button", "acct-details-toggle-checking") == nil || byTestID(x.f, "button", "acct-details-toggle-locked") == nil {
			t.Fatalf("account host did not render its rows: %s", x.f.Text())
		}
		if !strings.Contains(byTestID(x.f, "button", "acct-balance-btn-checking").Text(), "91.00") {
			t.Fatalf("compact row shows the wrong balance: %s", x.f.Text())
		}
		if byTestID(x.f, "div", "acct-details-checking") != nil {
			t.Fatal("details panel is open before its disclosure was used")
		}
		// Detail facts are prepared only for an opened disclosure.
		if accountDetailFactsCache.Len() != 0 {
			t.Fatal("compact host prepared account detail facts before any disclosure opened")
		}
	})

	t.Run("rows keep their sort order", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		var order []string
		for _, row := range x.f.AllByTag("div") {
			if id := row.Attr("data-testid"); strings.HasPrefix(id, "acct-row-") {
				order = append(order, id)
			}
		}
		if want := []string{"acct-row-checking", "acct-row-locked"}; !reflect.DeepEqual(order, want) {
			t.Fatalf("row order = %v, want %v", order, want)
		}
	})

	t.Run("opening a row shows its history, projection and forward line", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		x.click(t, "acct-details-toggle-checking")
		want := accountflow.Polyline(accountflow.BalanceSeries(x.checking, x.app.TransactionsByAccount(x.checking.ID), x.now, 90), sparklineW, sparklineH, 2)
		if got := x.chartPoints(); got != want {
			t.Fatalf("balance chart points = %q, want %q", got, want)
		}
		if byTestID(x.f, "div", "acct-proj-checking") == nil {
			t.Fatal("projected-low line is missing")
		}
		if byTestID(x.f, "span", "acct-fwd-negative-checking") == nil {
			t.Fatalf("90-day forward line missed the recurring debit due after day 30: %s", x.f.Text())
		}
	})

	t.Run("range picker redraws the chart and survives collapse and reopen", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		x.click(t, "acct-details-toggle-checking")
		ninetyDays := x.chartPoints()
		x.click(t, "acct-range-all-checking")
		allHistory := x.chartPoints()
		if allHistory == "" || allHistory == ninetyDays {
			t.Fatalf("all-history chart %q should differ from the 90-day chart %q", allHistory, ninetyDays)
		}
		x.click(t, "acct-details-toggle-checking")
		x.click(t, "acct-details-toggle-checking")
		if got := byTestID(x.f, "button", "acct-range-all-checking").Attr("aria-checked"); got != "true" {
			t.Fatalf("reopened panel lost the selected range: aria-checked=%q", got)
		}
		if got := x.chartPoints(); got != allHistory {
			t.Fatalf("reopened chart %q differs from the selected range %q", got, allHistory)
		}
	})

	t.Run("projection explanation lists near-term drivers only", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		x.click(t, "acct-details-toggle-checking")
		x.click(t, "acct-proj-toggle-checking")
		drivers := byTestID(x.f, "div", "acct-proj-drivers-checking")
		if drivers == nil || !strings.Contains(drivers.Text(), "Rent") {
			t.Fatal("30-day projection explanation lost its near-term driver")
		}
		if strings.Contains(drivers.Text(), "Later reserve") {
			t.Fatal("30-day projection explanation included a flow beyond 30 days")
		}
	})

	t.Run("a ledger change refreshes the balance and keeps the open panel and range", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		x.click(t, "acct-details-toggle-checking")
		x.click(t, "acct-range-all-checking")
		if err := x.app.Store().PutTransaction(domain.Transaction{ID: "new", AccountID: x.checking.ID, Date: x.now, Amount: money.New(-1000, "USD")}); err != nil {
			t.Fatal(err)
		}
		uistate.BumpDataRevision()
		x.f.Stabilize()
		if !strings.Contains(byTestID(x.f, "button", "acct-balance-btn-checking").Text(), "81.00") {
			t.Fatalf("balance did not refresh after the ledger changed: %s", x.f.Text())
		}
		if byTestID(x.f, "div", "acct-details-checking") == nil {
			t.Fatal("ledger change closed the details panel")
		}
		if got := byTestID(x.f, "button", "acct-range-all-checking").Attr("aria-checked"); got != "true" {
			t.Fatalf("ledger change lost the selected range: aria-checked=%q", got)
		}
	})

	t.Run("balance figure opens the set-balance editor", func(t *testing.T) {
		x := newLazyAccountsFixture(t)
		editor := state.NewGlobalAtom("accounts:edit", uistate.AccountEdit{})
		previous := editor.Get()
		t.Cleanup(func() { editor.Set(previous) })
		x.click(t, "acct-balance-btn-checking")
		if got := editor.Get(); got.ID != x.checking.ID || got.Mode != uistate.AcctEditModeSetBal {
			t.Fatalf("balance action opened %+v", got)
		}
	})
}

func TestAccountDetailContextIdentity(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD"}
	account := domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(1000, "USD")}
	newApp := func(t *testing.T) *appstate.App {
		t.Helper()
		app := primaryAccountsGoalsApp(t)
		clearAccountDetailCaches()
		t.Cleanup(clearAccountDetailCaches)
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
		if err := app.Store().PutRecurring(domain.Recurring{ID: "intraday", Label: "Later today", AccountID: account.ID,
			Amount: money.New(-100, "USD"), NextDue: now.Add(time.Hour), Cadence: domain.CadenceMonthly}); err != nil {
			t.Fatal(err)
		}
		return app
	}

	t.Run("identical context reuses the prepared projection", func(t *testing.T) {
		app := newApp(t)
		context := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now)
		first := context.facts(account)
		if context.facts(account) != first {
			t.Fatal("identical context rebuilt the facts")
		}
		if len(first.Projection.Drivers) != 1 {
			t.Fatalf("projection drivers = %d, want the one recurring debit", len(first.Projection.Drivers))
		}
	})

	t.Run("the calendar day decides which occurrences count", func(t *testing.T) {
		app := newApp(t)
		first := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now).facts(account)
		later := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now.Add(2*time.Hour)).facts(account)
		if later != first {
			t.Fatal("a later time on the same day rebuilt the facts")
		}
		if len(later.Projection.Drivers) != 1 {
			t.Fatal("a bill due earlier today dropped out of today's projection")
		}
		nextDay := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now.AddDate(0, 0, 1)).facts(account)
		if nextDay == first {
			t.Fatal("a new day reused the previous day's projection")
		}
	})

	t.Run("rate changes do not share facts", func(t *testing.T) {
		app := newApp(t)
		first := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now).facts(account)
		changedRates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
		if changed := newAccountDetailContext(app, accountSnapshot(app), changedRates, "USD", now).facts(account); changed == first {
			t.Fatal("different rates shared one facts identity")
		}
	})

	t.Run("series is keyed by calendar day, not by time zone", func(t *testing.T) {
		app := newApp(t)
		series, _ := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now).series(account, "90d")
		otherZone := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.FixedZone("west", -5*60*60))
		zoned, _ := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", otherZone).series(account, "90d")
		if &series[0] == &zoned[0] {
			t.Fatal("contexts in different time zones shared one series slice")
		}
		if !reflect.DeepEqual(series, zoned) {
			t.Fatal("same calendar day produced different series in another time zone")
		}
		nextDay, _ := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now.AddDate(0, 0, 1)).series(account, "90d")
		if &series[0] == &nextDay[0] {
			t.Fatal("a new day reused the previous day's series")
		}
	})

	t.Run("app instances do not leak into each other", func(t *testing.T) {
		app := newApp(t)
		other := primaryAccountsGoalsApp(t)
		changed := account
		changed.OpeningBalance.Amount = 2000
		if err := other.PutAccount(changed); err != nil {
			t.Fatal(err)
		}
		// The global revision can be identical across these apps. Instance identity
		// must still separate balances, history and projection facts.
		left := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now)
		right := newAccountDetailContext(other, accountSnapshot(other), rates, "USD", now)
		if got := left.facts(app.Accounts()[0]).Projection.Start; got != 1000 {
			t.Fatalf("left projection starts at %d, want 1000", got)
		}
		if got := right.facts(changed).Projection.Start; got != 2000 {
			t.Fatalf("right projection starts at %d, want 2000", got)
		}
	})

	t.Run("a store change does not reuse old facts", func(t *testing.T) {
		app := newApp(t)
		before := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now).facts(app.Accounts()[0])
		if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 3}}); err != nil {
			t.Fatal(err)
		}
		if after := newAccountDetailContext(app, accountSnapshot(app), rates, "USD", now).facts(app.Accounts()[0]); after == before {
			t.Fatal("store revision retained old account facts")
		}
	})
}

func TestAccountsHostRejectsHistoryAndProjectionForInvalidLedgerBalance(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	resetAccountsListFilters(t)
	clearAccountDetailCaches()
	t.Cleanup(clearAccountDetailCaches)
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
	t.Cleanup(f.Cleanup)
	f.Render(ui.CreateElement(Accounts))
	f.Stabilize()
	byTestID(f, "button", "acct-details-toggle-invalid").Click()
	f.Stabilize()

	t.Run("history says it is unavailable and draws no chart", func(t *testing.T) {
		if byTestID(f, "div", "acct-history-unavailable-invalid") == nil || byTestID(f, "svg", "acct-spark-invalid") != nil {
			t.Fatalf("invalid exact balance became a valid chart: %s", f.Text())
		}
	})
	t.Run("projections say they are unavailable", func(t *testing.T) {
		if byTestID(f, "div", "acct-proj-unavailable-invalid") == nil || byTestID(f, "p", "acct-fwd-unavailable-invalid") == nil {
			t.Fatalf("invalid exact balance became a plausible zero projection: %s", f.Text())
		}
	})
}

func TestAccountsHostShowsUnavailableForProjectionOverflowAcrossConsumers(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	resetAccountsListFilters(t)
	clearAccountDetailCaches()
	t.Cleanup(clearAccountDetailCaches)
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
	t.Cleanup(accounts.Cleanup)

	t.Run("the list shows both projections as unavailable", func(t *testing.T) {
		accounts.Render(ui.CreateElement(Accounts))
		accounts.Stabilize()
		byTestID(accounts, "button", "acct-details-toggle-overflow").Click()
		accounts.Stabilize()
		if byTestID(accounts, "div", "acct-proj-unavailable-overflow") == nil || byTestID(accounts, "p", "acct-fwd-unavailable-overflow") == nil {
			t.Fatalf("projection overflow appeared as a healthy or plausible amount: %s", accounts.Text())
		}
	})
	t.Run("the balance editor shows the projection as unavailable", func(t *testing.T) {
		accounts.Render(ui.CreateElement(AccountEditForm, AccountEditFormProps{AccountID: account.ID, Mode: uistate.AcctEditModeSetBal}))
		accounts.Stabilize()
		projected := byTestID(accounts, "span", "acct-bal-projected")
		if projected == nil || projected.Text() != uistate.T("calculation.unavailable") {
			t.Fatalf("editor hid projection arithmetic failure: %s", accounts.Text())
		}
	})
}
