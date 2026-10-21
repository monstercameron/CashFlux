// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func creditHistoryHostFixture(t *testing.T) (*appstate.App, domain.Account) {
	t.Helper()
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	t.Cleanup(func() { css.SetSink(previousCSS) })
	app := primaryAccountsGoalsApp(t)
	account := domain.Account{ID: "history-card", Name: "History card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.Zero("USD"), CreditLimit: money.New(10000, "USD")}
	// Fixture setup inserts a valid account without creating extra automatic
	// balance snapshots; production reads and editor writes remain unmodified.
	if err := app.Store().PutAccount(account); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	return app, account
}
func creditHistoryStorePoint(t *testing.T, app *appstate.App, id string, amount int64, code string, day int) {
	t.Helper()
	if err := app.Store().PutBalanceSnapshot(domain.BalanceSnapshot{ID: id, AccountID: "history-card", BalanceMinor: amount, Currency: code, AsOf: time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
}
func creditHistoryExpectPoint(t *testing.T, f *render.Fixture, id, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		node := byTestID(f, "div", "credit-history-point-"+id)
		if node != nil && strings.Contains(node.Text(), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("point%s wants%q: %s", id, want, f.Text())
}
func TestCreditHistoryHostTracksRatesAndIndependentUnavailablePoints(t *testing.T) {
	app, _ := creditHistoryHostFixture(t)
	creditHistoryStorePoint(t, app, "eur", -1000, "EUR", 1)
	creditHistoryStorePoint(t, app, "over", -15000, "USD", 2)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	// Actual page owns its revision subscription. The test parent adds none.
	f.Render(ui.CreateElement(CreditScreen))
	creditHistoryExpectPoint(t, f, "eur", "20%")
	creditHistoryExpectPoint(t, f, "over", "150%")
	if byTestID(f, "p", "credit-history-basis") == nil {
		t.Fatal("current-limit/current-FX basis missing")
	}
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditHistoryExpectPoint(t, f, "eur", "30%")
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditHistoryExpectPoint(t, f, "eur", "Unavailable")
	creditHistoryExpectPoint(t, f, "over", "150%")
	if byTestID(f, "div", "credit-history-bar-eur") != nil || byTestID(f, "p", "credit-history-unavailable") == nil {
		t.Fatal("unknown FX became zero bar or lost availability caption")
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	creditHistoryExpectPoint(t, f, "eur", "20%")
	if byTestID(f, "p", "credit-history-unavailable") != nil {
		t.Fatal("restored rates retained stale error")
	}
	creditHistoryStorePoint(t, app, "minimum", math.MinInt64, "USD", 3)
	uistate.BumpDataRevision()
	creditHistoryExpectPoint(t, f, "minimum", "Unavailable")
	if byTestID(f, "div", "credit-history-bar-minimum") != nil {
		t.Fatal("unrepresentable magnitude fabricated a bar")
	}
}
func TestCreditHistoryHostPreservesLatestEightAndLimitEditorRevision(t *testing.T) {
	app, account := creditHistoryHostFixture(t)
	for day := 10; day >= 1; day-- {
		code := "USD"
		if day == 10 {
			code = ""
		}
		creditHistoryStorePoint(t, app, fmt.Sprintf("point-%02d", day), -1783, code, day)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(CreditScreen))
	creditHistoryExpectPoint(t, f, "point-03", "18%")
	ids := []string{}
	for _, node := range f.AllByTag("div") {
		id := node.Attr("data-testid")
		if strings.HasPrefix(id, "credit-history-point-") {
			ids = append(ids, id)
		}
	}
	if len(ids) != 8 || ids[0] != "credit-history-point-point-03" || ids[7] != "credit-history-point-point-10" {
		t.Fatalf("latest-eight chronology: %v", ids)
	}
	if !strings.Contains(f.Text(), "Latest 8 of 10 balance records") || !strings.Contains(f.Text(), "Older records without a currency") {
		t.Fatalf("selection or legacy basis omitted: %s", f.Text())
	}
	field := byTestID(f, "input", "credit-limit-edit-"+account.ID)
	if field == nil {
		t.Fatal("actual limit editor missing")
	}
	field.Input("200.00")
	f.Stabilize()
	field = byTestID(f, "input", "credit-limit-edit-"+account.ID)
	field.Dispatch("onblur", render.Event{})
	creditHistoryExpectPoint(t, f, "point-03", "9%")
	stored, found, err := app.Store().GetAccount(account.ID)
	if err != nil || !found || stored.CreditLimit != money.New(20000, "USD") || stored.Name != account.Name {
		t.Fatalf("limit editor save changed wrong fields: %+v %v", stored, err)
	}
}
func TestCreditHistoryAdapterReadFailureIsNotEmptyHistory(t *testing.T) {
	app, account := creditHistoryHostFixture(t)
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	history := buildUtilTrend(app, account.ID)
	if history.Err == nil || history.MissingLimit || history.Ready {
		t.Fatalf("read failure hidden as empty: %+v", history)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node { return creditHistoryNode(history) }))
	if byTestID(f, "p", "credit-history-unavailable") == nil || strings.Contains(f.Text(), "Not enough history") {
		t.Fatalf("source failure misrepresented: %s", f.Text())
	}
}
