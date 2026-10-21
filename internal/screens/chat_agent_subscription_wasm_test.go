// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func subscriptionTestApp(t *testing.T) (*appstate.App, time.Time) {
	t.Helper()
	app := agrApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	old := assistantSubscriptionNow
	assistantSubscriptionNow = func() time.Time { return now }
	t.Cleanup(func() { assistantSubscriptionNow = old })
	for _, code := range []string{"EUR", "JPY"} {
		a := netWorthFixtureAccount("sub-"+code, code, 0, false)
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	return app, now
}

func subscriptionTestSchedule(t *testing.T, app *appstate.App, now time.Time, id string, amount int64, cadence domain.RecurringCadence) domain.Recurring {
	t.Helper()
	r := domain.Recurring{ID: id, Label: "Streamly", AccountID: "sub-EUR", Amount: money.New(amount, "EUR"), Cadence: cadence, NextDue: now.AddDate(0, 1, 0)}
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	return r
}

func subscriptionTestCharge(t *testing.T, app *appstate.App, id, code string, at time.Time, amount int64) {
	t.Helper()
	account := "sub-" + code
	if code == "USD" {
		account = "a-check"
	}
	if err := app.PutTransaction(domain.Transaction{ID: id, AccountID: account, Payee: "Streamly", Desc: "Provider bill", Date: at, Amount: money.New(amount, code)}); err != nil {
		t.Fatal(err)
	}
}

func TestAssistantSubscriptionUsesExactAnnualCurrencyAndFreshActiveSchedule(t *testing.T) {
	app, now := subscriptionTestApp(t)
	r := subscriptionTestSchedule(t, app, now, "annual", -12001, domain.CadenceYearly)
	subscriptionTestCharge(t, app, "sub-prior", "EUR", now.AddDate(-2, 0, 0), -10000)
	tool := assistantFinanceTestTool(t, agToolsCoach(app, "USD", currency.Rates{Base: "USD"}), "prepare_subscription_call")
	raw := json.RawMessage(`{"name":"Streamly"}`)
	out := tool.run(raw)
	for _, want := range []string{"120.01 EUR", "10.00 EUR", "monthly equivalent", "nonmonthly schedule", "Household subscription review", "1 eligible historical gross charges"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "$") || strings.Contains(out, "120.00 EUR") || strings.Contains(out, "price went up") {
		t.Fatalf("annual/currency/cadence misrepresented: %s", out)
	}
	r.Amount = money.New(-12002, "EUR")
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	if out = tool.run(raw); !strings.Contains(out, "120.02 EUR") {
		t.Fatalf("same handler retained old schedule: %s", out)
	}
	r.Paused = true
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	if !strings.Contains(out, "Monthly and annual costs are unknown") || strings.Contains(out, "costs you") || strings.Contains(out, "paying 100.00 EUR a month") {
		t.Fatalf("paused schedule/historical cost inferred: %s", out)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	if !strings.HasPrefix(out, "Subscription preparation unavailable: read subscription settings:") || strings.Contains(out, "Nothing matching") {
		t.Fatalf("store failure hidden: %s", out)
	}
}

func TestAssistantSubscriptionMixedCurrencyEvidenceUsesFreshFXWithoutPriceRise(t *testing.T) {
	app, now := subscriptionTestApp(t)
	subscriptionTestSchedule(t, app, now, "monthly", -2000, domain.CadenceMonthly)
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for i, amount := range []int64{-2000, -3000, -4000} {
		subscriptionTestCharge(t, app, []string{"sub-one", "sub-two", "sub-three"}[i], "USD", now.AddDate(0, -3+i, 0), amount)
	}
	tool := assistantFinanceTestTool(t, agToolsCoach(app, "JPY", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 999}}), "prepare_subscription_call")
	raw := json.RawMessage(`{"name":"Streamly"}`)
	out := tool.run(raw)
	for _, want := range []string{"240.00 EUR", "20.00 EUR", "mixed-currency history uses current FX", "not historical rates"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "price went up") || strings.Contains(out, "JPY") {
		t.Fatalf("converted history created nominal rise/captured currency: %s", out)
	}
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	out = tool.run(raw)
	if !strings.HasPrefix(out, "Subscription preparation unavailable:") || !strings.Contains(out, "FX:") || strings.Contains(out, "What to say:") {
		t.Fatalf("missing FX supplied partial sheet: %s", out)
	}
}

func TestAssistantSubscriptionAmbiguityOverflowAndUnscheduledJPYRemainExplicit(t *testing.T) {
	app, now := subscriptionTestApp(t)
	subscriptionTestSchedule(t, app, now, "first", -1000, domain.CadenceMonthly)
	r := subscriptionTestSchedule(t, app, now, "second", -2000, domain.CadenceMonthly)
	tool := assistantFinanceTestTool(t, agToolsCoach(app, "USD", currency.Rates{Base: "USD"}), "prepare_subscription_call")
	out := tool.run(json.RawMessage(`{"name":"Streamly"}`))
	if !strings.Contains(out, "ambiguous subscription") || !strings.Contains(out, "recurring_id second") {
		t.Fatalf("arbitrary provider selected: %s", out)
	}
	out = tool.run(json.RawMessage(`{"recurring_id":"second"}`))
	if !strings.Contains(out, "240.00 EUR") || strings.Contains(out, "unavailable") {
		t.Fatalf("exact ID did not resolve: %s", out)
	}
	r.Amount = money.New(-math.MaxInt64, "EUR")
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"recurring_id":"second"}`))
	if !strings.Contains(out, "Subscription preparation unavailable:") || !strings.Contains(out, "overflow") || strings.Contains(out, "What to say:") {
		t.Fatalf("annual overflow became claim: %s", out)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "sub-jpy", AccountID: "sub-JPY", Payee: "Local service", Desc: "One historical purchase", Date: now.AddDate(-1, 0, 0), Amount: money.New(-1234, "JPY")}); err != nil {
		t.Fatal(err)
	}
	out = tool.run(json.RawMessage(`{"name":"Local service"}`))
	if !strings.Contains(out, "Amounts in JPY") || !strings.Contains(out, "Monthly and annual costs are unknown") || strings.Contains(out, "12.34") || strings.Contains(out, "a month, which is") {
		t.Fatalf("JPY history invented cadence/cost: %s", out)
	}
}
