// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/insightsperiod"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type assistantBriefingFixtureProps struct {
	App     *appstate.App
	Scope   scope.ReportScope
	Period  insightsperiod.Period
	Now     time.Time
	Capture func(*assistantview.Briefing)
}

func assistantBriefingFixture(p assistantBriefingFixtureProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	value := useAssistantBriefing(p.App, p.Scope, p.Period, p.Now, "Uncategorized")
	p.Capture(value)
	return P(fmt.Sprint(value.Spent))
}

func newAssistantBriefingApp(t *testing.T, multiplier int64) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	for i, institution := range []string{"A B", "A", "B"} {
		id := fmt.Sprint(i)
		if err := app.PutAccount(domain.Account{ID: id, Name: id, Institution: institution,
			OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR"}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutTransaction(domain.Transaction{ID: id, AccountID: id, Desc: "Market",
			Date: time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), Amount: money.New(-int64(i+1)*100*multiplier, "EUR")}); err != nil {
			t.Fatal(err)
		}
	}
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestAssistantBriefingMemoTracksScopePeriodAndCalendar(t *testing.T) {
	app := newAssistantBriefingApp(t, 1)
	var got *assistantview.Briefing
	props := assistantBriefingFixtureProps{App: app, Scope: scope.ReportScope{Institutions: []string{"A B"}},
		Period: insightsperiod.ThisMonth, Now: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Capture: func(value *assistantview.Briefing) { got = value }}
	f := render.New(t)
	defer f.Cleanup()
	renderValue := func() {
		f.Render(ui.CreateElement(assistantBriefingFixture, props))
		f.Stabilize()
	}
	renderValue()
	first := got
	if got.Spent != 200 {
		t.Fatalf("single institution spend = %d", got.Spent)
	}
	renderValue()
	if got != first {
		t.Fatal("presentation rerender recomputed the financial snapshot")
	}
	props.Scope = scope.ReportScope{Institutions: []string{"A", "B"}}
	renderValue()
	if got == first || got.Spent != 1000 {
		t.Fatalf("multiple institutions reused the space-containing institution: %+v", got)
	}
	props.Period = insightsperiod.LastMonth
	renderValue()
	if got.Spent != 0 || got.Window.Period != insightsperiod.LastMonth {
		t.Fatal("period selection retained current-month spend")
	}
	props.Now = props.Now.AddDate(0, 1, 0)
	renderValue()
	if got.Spent != 1000 {
		t.Fatalf("month boundary retained stale spend: %d", got.Spent)
	}
}

func TestAssistantBriefingMemoTracksAppFXAndBrowserRevision(t *testing.T) {
	firstApp, otherApp := newAssistantBriefingApp(t, 1), newAssistantBriefingApp(t, 2)
	if firstApp.Rev() != otherApp.Rev() {
		t.Fatal("fixture needs equal store revisions")
	}
	var got *assistantview.Briefing
	props := assistantBriefingFixtureProps{App: firstApp, Period: insightsperiod.ThisMonth,
		Now: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC), Capture: func(value *assistantview.Briefing) { got = value }}
	f := render.New(t)
	defer f.Cleanup()
	renderValue := func() {
		f.Render(ui.CreateElement(assistantBriefingFixture, props))
		f.Stabilize()
	}
	renderValue()
	if got.Spent != 1200 {
		t.Fatal(got.Spent)
	}
	props.App = otherApp
	renderValue()
	if got.Spent != 2400 {
		t.Fatalf("different app reused equal revision: %d", got.Spent)
	}
	settings := otherApp.Settings()
	settings.FXRates["EUR"] = 3
	if err := otherApp.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	renderValue()
	if got.Spent != 3600 {
		t.Fatalf("FX change retained old converted amount: %d", got.Spent)
	}
	previous, revision := got, otherApp.Rev()
	uistate.BumpDataRevision()
	f.Stabilize()
	if got == previous || otherApp.Rev() != revision || got.Spent != 3600 {
		t.Fatal("browser revision did not refresh the financial snapshot independently")
	}
}

func TestSpendingAnomaliesMemoSeparatesExplicitAppAndScope(t *testing.T) {
	firstApp, otherApp := newAssistantBriefingApp(t, 1), newAssistantBriefingApp(t, 2)
	if firstApp.Rev() != otherApp.Rev() {
		t.Fatal("fixture needs equal store revisions")
	}
	categories := []domain.Category{{ID: "food", Name: "Food"}}
	rates := currency.Rates{Base: "USD"}
	current := dateutil.MonthStart(time.Now())
	steady := make([]domain.Transaction, 4)
	for i := range steady {
		steady[i] = domain.Transaction{ID: fmt.Sprint(i), CategoryID: "food", Amount: money.New(-1000, "USD"), Date: dateutil.AddMonths(current, i-3)}
	}
	spike := append([]domain.Transaction(nil), steady...)
	spike[3].Amount = money.New(-10000, "USD")
	firstScope := "dash-highlight|" + acctSig([]domain.Account{{ID: "first"}})
	otherScope := "dash-highlight|" + acctSig([]domain.Account{{ID: "other"}})
	first := detectSpendingAnomaliesMemo(firstApp, firstScope, spike, categories, rates)
	if len(first) != 1 || first[0].Current != 10000 {
		t.Fatalf("spike missing: %+v", first)
	}
	if got := detectSpendingAnomaliesMemo(firstApp, otherScope, steady, categories, rates); len(got) != 0 {
		t.Fatalf("scope reused another account's spike: %+v", got)
	}
	if got := detectSpendingAnomaliesMemo(otherApp, firstScope, steady, categories, rates); len(got) != 0 {
		t.Fatalf("different app at equal revision reused the spike: %+v", got)
	}
	first[0].Current = 1
	if got := detectSpendingAnomaliesMemo(firstApp, firstScope, spike, categories, rates); len(got) != 1 || got[0].Current != 10000 {
		t.Fatalf("caller mutated cached anomaly: %+v", got)
	}
	if got := detectSpendingAnomaliesMemo(nil, firstScope, steady, categories, rates); len(got) != 0 {
		t.Fatalf("nil-app derivation reused cached data: %+v", got)
	}
	if got := detectSpendingAnomaliesMemo(nil, firstScope, spike, categories, rates); len(got) != 1 {
		t.Fatalf("nil-app derivation did not use supplied transactions: %+v", got)
	}
}
