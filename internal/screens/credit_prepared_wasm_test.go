// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"reflect"
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

func creditPreparedFixture(t *testing.T) (*appstate.App, domain.Account, domain.Account) {
	t.Helper()
	app, usd := creditHistoryHostFixture(t)
	usd.OpeningBalance = money.New(-80000, "USD")
	usd.CreditLimit = money.New(100000, "USD")
	usd.BalanceAsOf = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	usd.DueDayOfMonth = 15
	eur := usd
	eur.ID, eur.Name, eur.Currency = "eur-card", "Euro card", "EUR"
	eur.OpeningBalance, eur.CreditLimit = money.New(-50000, "EUR"), money.New(100000, "EUR")
	for _, a := range []domain.Account{usd, eur} {
		if err := app.Store().PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	return app, usd, eur
}

func creditPreparedWait(t *testing.T, f *render.Fixture, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Credit host did not reach expected state: %s", f.Text())
}

func TestCreditPreparedHostSharesCanonicalFXFactsAndContext(t *testing.T) {
	app, _, eur := creditPreparedFixture(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	// The actual route owns its subscription; the fixture parent subscribes to none.
	f.Render(ui.CreateElement(CreditScreen))
	creditPreparedWait(t, f, func() bool { return byTestID(f, "div", "credit-score") != nil })
	model := prepareCredit(app, time.Now(), true, true)
	if model.UtilPct != 60 || !model.ProxyKnown || byTestID(f, "span", "credit-aggregate-util").Text() != "60%" || byTestID(f, "div", "credit-score").Text() != fmt.Sprint(model.ProxyScore) {
		t.Fatalf("visual/canonical facts disagree: %+v %s", model, f.Text())
	}
	pay := byTestID(f, "p", "credit-card-payment-"+eur.ID)
	if pay == nil || !strings.Contains(pay.Text(), fmtMoney(money.New(20000, "EUR"))) || strings.Contains(pay.Text(), fmtMoney(money.New(20000, "USD"))) {
		t.Fatalf("native payment currency lost: %s", f.Text())
	}
	context := creditContextString(app)
	if !strings.Contains(context, "Scope: household; base currency: USD;") || !strings.Contains(context, "Overall utilization: 60%") || !strings.Contains(context, fmt.Sprintf("Overall score: %d/100", model.ProxyScore)) || !strings.Contains(context, uistate.T("credit.adviceLowerUtil", fmtMoney(money.New(20000, "EUR")), eur.Name, 30)) {
		t.Fatalf("AI facts differ from visible canonical facts: %s", context)
	}

	settings := app.Settings()
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditPreparedWait(t, f, func() bool { return byTestID(f, "div", "credit-score-unavailable") != nil })
	if byTestID(f, "div", "credit-score") != nil || byTestID(f, "span", "credit-aggregate-util").Text() != uistate.T("credit.modelUnavailable") || byTestID(f, "span", "credit-card-util-"+eur.ID).Text() != "50%" || byTestID(f, "div", "cf-var-age") == nil || byTestID(f, "div", "cf-var-ontime") == nil || strings.Contains(f.Text(), uistate.T("credit.adviceAddLimit", eur.Name)) {
		t.Fatalf("unknown FX became zero/no-limit or discarded independent facts: %s", f.Text())
	}
	context = creditContextString(app)
	if !strings.Contains(context, "Overall score: unavailable") || !strings.Contains(context, "score impact unavailable") || !strings.Contains(context, fmtMoney(money.New(20000, "EUR"))) {
		t.Fatalf("AI context fabricated score or dropped native advice: %s", context)
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditPreparedWait(t, f, func() bool {
		return byTestID(f, "div", "credit-score") != nil && byTestID(f, "div", "credit-score-unavailable") == nil
	})
}

func TestCreditPreparedSummarySkipsDetailsAndSharesHeadline(t *testing.T) {
	app, _, _ := creditPreparedFixture(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(CreditHealthPanel, CreditHealthPanelProps{Summary: true}))
	summary := prepareCredit(app, time.Now(), false, false)
	full := prepareCredit(app, time.Now(), true, true)
	if summary.ProxyScore != full.ProxyScore || summary.UtilPct != full.UtilPct || summary.BalanceFolds != 1 || summary.HistoryBuilds != 0 || len(summary.Cards) != 0 || len(summary.Advice) != 0 {
		t.Fatalf("summary prepared details or disagreed: %+v %+v", summary, full)
	}
	if byTestID(f, "div", "credit-score") == nil || byTestID(f, "div", "credit-score").Text() != fmt.Sprint(full.ProxyScore) || byTestID(f, "p", "credit-handoff") == nil || byTestID(f, "input", "credit-limit-edit-history-card") != nil || byTestID(f, "div", "credit-history") != nil || byTestID(f, "div", "credit-advice") != nil {
		t.Fatalf("summary emitted deep children or lost headline: %s", f.Text())
	}
}

func TestCreditPreparedKeyedLimitDraftPreservesLiveAccountAndRejectsChangedCurrency(t *testing.T) {
	app, usd, eur := creditPreparedFixture(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(CreditScreen))
	creditPreparedWait(t, f, func() bool { return byTestID(f, "input", "credit-limit-edit-"+eur.ID) != nil })
	byTestID(f, "input", "credit-limit-edit-"+eur.ID).Input("1200.25")
	f.Stabilize()
	// Insertion changes row positions; keyed card/editor identity must retain this draft.
	first := usd
	first.ID, first.Name = "aaa-first", "A first card"
	if err := app.Store().PutAccount(first); err != nil {
		t.Fatal(err)
	}
	eur.Name = "Renamed while editing"
	if err := app.Store().PutAccount(eur); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditPreparedWait(t, f, func() bool { return strings.Contains(f.Text(), eur.Name) })
	field := byTestID(f, "input", "credit-limit-edit-"+eur.ID)
	field.Dispatch("onblur", render.Event{})
	creditPreparedWait(t, f, func() bool {
		stored, _, _ := app.Store().GetAccount(eur.ID)
		return stored.CreditLimit == money.New(120025, "EUR")
	})
	stored, found, err := app.Store().GetAccount(eur.ID)
	if err != nil || !found || stored.Name != eur.Name {
		t.Fatalf("limit save overwrote live metadata: %+v %v", stored, err)
	}
	// The same editor's draft cannot silently retarget when the currency changes.
	byTestID(f, "input", "credit-limit-edit-"+eur.ID).Input("1300.00")
	f.Stabilize()
	stored.Currency, stored.OpeningBalance, stored.CreditLimit = "USD", money.Zero("USD"), money.New(90000, "USD")
	if err := app.Store().PutAccount(stored); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditPreparedWait(t, f, func() bool {
		return strings.Contains(byTestID(f, "p", "credit-card-balances-"+eur.ID).Text(), fmtMoney(money.New(90000, "USD")))
	})
	if bound := byTestID(f, "span", "credit-limit-currency-"+eur.ID); bound == nil || bound.Text() != "EUR" {
		t.Fatalf("payment-nudge removal remounted/rebound the EUR draft: %s", f.Text())
	}
	byTestID(f, "input", "credit-limit-edit-"+eur.ID).Dispatch("onblur", render.Event{})
	creditPreparedWait(t, f, func() bool { return byTestID(f, "span", "credit-limit-error-"+eur.ID) != nil })
	after, _, err := app.Store().GetAccount(eur.ID)
	if err != nil || !reflect.DeepEqual(after, stored) {
		t.Fatalf("stale currency draft wrote account: %+v / %+v %v", after, stored, err)
	}
}

func TestCreditPreparedReadFailureIsUnavailableAcrossVisualAndContext(t *testing.T) {
	app, _, _ := creditPreparedFixture(t)
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	for _, node := range []ui.Node{ui.CreateElement(CreditScreen), ui.CreateElement(CreditHealthPanel, CreditHealthPanelProps{Summary: true})} {
		f := render.New(t)
		f.Render(node)
		if byTestID(f, "section", "credit-source-unavailable") == nil || byTestID(f, "div", "credit-score") != nil || strings.Contains(f.Text(), uistate.T("credit.emptyBody")) {
			t.Errorf("source failure masqueraded as empty/zero: %s", f.Text())
		}
		f.Cleanup()
	}
	if context := creditContextString(app); !strings.Contains(context, "Credit data unavailable") || strings.Contains(context, "Overall score:") {
		t.Fatalf("unread context fabricated a score: %s", context)
	}
}
