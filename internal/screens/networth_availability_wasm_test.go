// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func netWorthFixtureAccount(id, code string, amount int64, liability bool) domain.Account {
	class := domain.ClassAsset
	kind := domain.TypeChecking
	if liability {
		class = domain.ClassLiability
		kind = domain.TypeOther
	}
	return domain.Account{ID: id, Name: "Private " + id, Currency: code, Class: class, Type: kind,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, OpeningBalance: money.New(amount, code)}
}

func TestPrimaryNetWorthHeroAccountsAvailabilityTransitions(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := primaryAccountsGoalsApp(t)
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	bank, debt := netWorthFixtureAccount("bank", "USD", 10000, false), netWorthFixtureAccount("debt", "EUR", -1000, true)
	for _, account := range []domain.Account{bank, debt} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	var lens state.Atom[scope.ReportScope]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		_ = uistate.UseDataRevision().Get()
		lens = uistate.UseActiveScope()
		s := app.Settings()
		return Div(ui.CreateElement(dashboardHero), ui.CreateElement(acctSummaryWidget, acctSummaryProps{
			App: app, Base: "USD", Rates: currency.Rates{Base: "USD", Rates: s.FXRates}}))
	}))
	lens.Set(scope.ReportScope{})
	f.Stabilize()
	assertFigure := func(id, want string) {
		t.Helper()
		node := byTestID(f, "div", id)
		if node == nil || !strings.Contains(node.Text(), want) {
			t.Fatalf("%s expected %q: %s", id, want, f.Text())
		}
	}
	for _, id := range []string{"hero-networth-value", "acct-networth-value"} {
		assertFigure(id, "100.00")
	}
	for _, id := range []string{"hero-networth-note", "acct-networth-note"} {
		node := byTestID(f, "p", id)
		if node == nil || !strings.Contains(node.Text(), "Partial total") || !strings.Contains(node.Text(), "EUR") || !strings.Contains(node.Text(), "1 account") {
			t.Fatalf("missing partial disclosure %s: %s", id, f.Text())
		}
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	for _, id := range []string{"hero-networth-value", "acct-networth-value"} {
		assertFigure(id, "80.00")
	}
	if byTestID(f, "p", "hero-networth-note") != nil || byTestID(f, "p", "acct-networth-note") != nil {
		t.Fatal("restored rates retained a stale partial warning")
	}
	debt.OpeningBalance = money.New(-1000, "GBP")
	// A persisted malformed balance (e.g. legacy input) bypasses form validation.
	if err := app.Store().PutAccount(debt); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	for _, id := range []string{"hero-networth-value", "acct-networth-value"} {
		assertFigure(id, "—")
	}
	for _, id := range []string{"hero-networth-note", "acct-networth-note"} {
		node := byTestID(f, "p", id)
		if node == nil || !strings.Contains(node.Text(), "unavailable") || strings.Contains(node.Text(), "Private") {
			t.Fatalf("fatal balance was not safely disclosed: %s", f.Text())
		}
	}
	// A scope change excludes the invalid account without changing the ledger.
	lens.Set(scope.ReportScope{AccountIDs: []string{bank.ID}})
	f.Stabilize()
	assertFigure("hero-networth-value", "100.00")
	assertFigure("acct-networth-value", "—")
	lens.Set(scope.ReportScope{})
	bank.OpeningBalance, debt.OpeningBalance = money.Zero("USD"), money.Zero("EUR")
	for _, account := range []domain.Account{bank, debt} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	for _, id := range []string{"hero-networth-value", "acct-networth-value"} {
		assertFigure(id, "0.00")
	}
	if byTestID(f, "p", "hero-networth-note") != nil || byTestID(f, "p", "acct-networth-note") != nil {
		t.Fatal("a genuine known zero was marked unavailable")
	}
}

func TestPrimaryNetWorthCacheSeparatesRatesScopeAndApp(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	accounts := []domain.Account{netWorthFixtureAccount("usd", "USD", 100, false), netWorthFixtureAccount("eur", "EUR", 200, false)}
	for _, account := range accounts {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	missing := netWorthSnapshot(app, app.Accounts(), nil, currency.Rates{Base: "USD"})
	full := netWorthSnapshot(app, app.Accounts(), nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	changed := netWorthSnapshot(app, app.Accounts(), nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 3}})
	scoped := netWorthSnapshot(app, accounts[:1], nil, currency.Rates{Base: "USD"})
	if !missing.Partial || !full.Complete() || full.Net.Amount != 500 || changed.Net.Amount != 700 || !scoped.Complete() || scoped.Net.Amount != 100 {
		t.Fatalf("cache reused incompatible rates or account set: %+v %+v %+v %+v", missing, full, changed, scoped)
	}
	other := primaryAccountsGoalsApp(t)
	for _, account := range accounts {
		account.OpeningBalance.Amount = 0
		if err := other.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	zero := netWorthSnapshot(other, other.Accounts(), nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if !zero.Complete() || zero.Net.Amount != 0 {
		t.Fatalf("another app reused cached household money: %+v", zero)
	}
}

func TestPrimaryAssistantNetWorthAvailabilityInSharedContext(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := primaryAccountsGoalsApp(t)
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates, settings.OpenAIKey = "USD", nil, ""
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	bank, debt := netWorthFixtureAccount("bank", "USD", 10000, false), netWorthFixtureAccount("debt", "EUR", -1000, true)
	for _, account := range []domain.Account{bank, debt} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	// Exercise the real composer while keeping the DOM-only markdown/composer
	// boundary local. No request is sent: the affordability path is native facts.
	doc := js.Global().Get("document")
	previousGet := doc.Get("getElementById")
	composer := js.Global().Get("Object").New()
	composer.Set("value", "")
	markdown := map[string]js.Value{}
	get := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].String() == "cf-chat-input" {
			return composer
		}
		if len(args) > 0 && (strings.HasPrefix(args[0].String(), "cf-md-") || strings.HasPrefix(args[0].String(), "cf-afford-")) {
			key := args[0].String()
			el, ok := markdown[key]
			if !ok {
				el = js.Global().Get("Object").New()
				el.Set("innerHTML", "")
				el.Set("dataset", js.Global().Get("Object").New())
				markdown[key] = el
			}
			return el
		}
		return js.Null()
	})
	doc.Set("getElementById", get)
	defer func() { doc.Set("getElementById", previousGet); get.Release() }()
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Assistant))
	f.Stabilize()
	byTestID(f, "button", "assistant-settings-toggle").Click()
	f.Stabilize()
	chip := byTestID(f, "button", "assistant-share-chip")
	if chip == nil {
		t.Fatalf("Assistant Ask host has no disclosure: %s", f.Text())
	}
	chip.Click()
	f.Stabilize()
	panel := func() string {
		t.Helper()
		node := byTestID(f, "div", "assistant-share-panel")
		if node == nil {
			t.Fatal("share disclosure closed during data change")
		}
		return node.Text()
	}
	text := panel()
	if !strings.Contains(text, "Partial total") || !strings.Contains(text, "EUR") || strings.Contains(text, "Private") {
		t.Fatalf("outgoing AI context hides exclusions or leaks account names: %s", text)
	}
	composer.Set("value", "Can I afford $500?")
	byTestID(f, "button", "assistant-send").Click()
	f.Stabilize()
	renderedUnavailable := false
	for _, el := range markdown {
		if strings.Contains(el.Get("innerHTML").String(), uistate.T("insights.affordNetWorthUnavailable")) {
			renderedUnavailable = true
		}
	}
	if !renderedUnavailable {
		t.Fatalf("partial net worth did not explain why affordability is unavailable: %s", f.Text())
	}
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-cf") == "afford-result" {
			t.Fatal("partial net worth produced an affordability recommendation")
		}
	}
	debt.OpeningBalance = money.New(-1000, "GBP")
	if err := app.Store().PutAccount(debt); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if text = panel(); !strings.Contains(text, "Net worth "+uistate.T("calculation.unavailable")) || strings.Contains(text, "Net worth 0.00") || strings.Contains(text, "Private") {
		t.Fatalf("fatal AI context misstates money: %s", text)
	}
	debt.OpeningBalance = money.New(-1000, "EUR")
	if err := app.PutAccount(debt); err != nil {
		t.Fatal(err)
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if text = panel(); !strings.Contains(text, "80.00") || strings.Contains(text, "Partial total") || strings.Contains(text, "Net worth "+uistate.T("calculation.unavailable")) {
		t.Fatalf("AI context did not recover when rates and balances were repaired: %s", text)
	}
}

func TestPrimaryNotificationNetWorthWinRequiresCompleteTotal(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	bank, debt := netWorthFixtureAccount("bank", "USD", 2_000_000, false), netWorthFixtureAccount("debt", "EUR", -100, true)
	for _, account := range []domain.Account{bank, debt} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	goal := domain.Goal{ID: "goal", Name: "Independent goal", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TargetAmount: money.New(100, "USD"), CurrentAmount: money.New(100, "USD")}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(notifWinsWidget, notifProps{App: app}))
	f.Stabilize()
	if strings.Contains(f.Text(), "Net worth") || !strings.Contains(f.Text(), goal.Name) {
		t.Fatalf("partial net worth falsely celebrated or hid independent win: %s", f.Text())
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(f.Text(), "Net worth") || !strings.Contains(f.Text(), goal.Name) {
		t.Fatalf("known net worth did not restore its win: %s", f.Text())
	}
	debt.OpeningBalance = money.New(-100, "GBP")
	if err := app.Store().PutAccount(debt); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if strings.Contains(f.Text(), "Net worth") || !strings.Contains(f.Text(), goal.Name) {
		t.Fatalf("fatal net worth left a stale win: %s", f.Text())
	}
}
