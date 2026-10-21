// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestRecurringPreparedFactsUseFreshFXAndKeepIndependentRows(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	settings, err := app.Store().GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for _, flow := range []domain.Recurring{
		{ID: "income", Label: "Paycheck", Amount: money.New(5000, "USD"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 1, 0)},
		{ID: "foreign-bill", Label: "Euro bill", Amount: money.New(-1000, "EUR"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 1, 0)},
	} {
		if err := app.PutRecurring(flow); err != nil {
			t.Fatal(err)
		}
	}
	view := computeRecurView(app, now)
	if view.MonthlyAvailable || view.MonthlyError == nil {
		t.Fatalf("missing FX looked like a complete monthly summary: %+v", view)
	}
	income := view.MonthlyByID["income"]
	if !income.MonthlyAvailable || income.MonthlyAmount != 5000 || !income.ShareAvailable || income.SharePercent != 100 {
		t.Fatalf("unrelated known income fact was hidden: %+v", income)
	}
	if view.MonthlyByID["foreign-bill"].MonthlyAvailable {
		t.Fatal("foreign amount without a rate was presented as a base-currency amount")
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(rhyHeroHost, rhythmView{recurView: view}))
	f.Stabilize()
	if got := f.Text(); !strings.Contains(got, uistate.T("common.notReady")) || strings.Contains(got, "$0.00") {
		t.Fatalf("hero did not mark the incomplete summary unavailable: %s", got)
	}

	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	view = computeRecurView(app, now)
	if !view.MonthlyAvailable || view.MonthlyIn != 5000 || view.MonthlyOut != 2000 || view.MonthlyNet != 3000 {
		t.Fatalf("fresh FX did not restore prepared totals: %+v", view)
	}
	if got := view.MonthlyByID["foreign-bill"]; !got.MonthlyAvailable || got.MonthlyAmount != -2000 {
		t.Fatalf("foreign row did not use refreshed FX: %+v", got)
	}
}

func rhyHeroHost(view rhythmView) ui.Node { return rhyHeroSection(view, nil) }
