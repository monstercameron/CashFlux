// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type budgetSummaryNativeProps struct {
	App      *appstate.App
	Window   period.Window
	Previous bool
}

func budgetSummaryNativeHost(props budgetSummaryNativeProps) ui.Node {
	previous := uistate.UseBudgetsLastMonth()
	if previous.Get() != props.Previous {
		previous.Set(props.Previous)
	}
	return ui.CreateElement(bud21FutureSummaryHarness, bud21FutureSummaryProps{
		App: props.App, Window: props.Window, Prefs: prefs.Prefs{BudgetIncomeMode: "fixed", MonthlyIncomeMinor: 500000},
	})
}

func budgetSummaryNativeApp(t *testing.T, cadence domain.Period) *appstate.App {
	t.Helper()
	app := bud2TestApp(t)
	t.Cleanup(func() { _ = app.Store().Close() })
	if err := app.Store().PutBudget(domain.Budget{ID: "summary", Name: "Food", CategoryID: "c-food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: cadence, Limit: money.New(120000, "USD")}); err != nil {
		t.Fatal(err)
	}
	return app
}

func budgetSummaryNativePut(t *testing.T, app *appstate.App, id string, month time.Month, amount int64, currency string) {
	t.Helper()
	if err := app.Store().PutTransaction(domain.Transaction{ID: id, CategoryID: "c-food", Date: time.Date(2024, month, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(amount, currency)}); err != nil {
		t.Fatal(err)
	}
}

func TestBudgetSummaryNativeHostUsesSelectedWindowAcrossCadenceAndPeriodChanges(t *testing.T) {
	app := budgetSummaryNativeApp(t, domain.PeriodYearly)
	budgetSummaryNativePut(t, app, "february", time.February, -2000, "USD")
	budgetSummaryNativePut(t, app, "march", time.March, -3000, "USD")
	window := period.NewWindow(period.Month, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC), time.Monday)
	v := computeBudgetView(app, "", window, prefs.Prefs{}, false)
	if v.TotalSpent != 5000 || v.SummaryActual != 3000 || !v.SummaryComplete {
		t.Fatalf("fixture must distinguish yearly card spend from selected-month union: %+v", v)
	}
	f := render.New(t)
	assertFigure := func(window period.Window, previous bool, want string) {
		t.Helper()
		f.Render(ui.CreateElement(budgetSummaryNativeHost, budgetSummaryNativeProps{App: app, Window: window, Previous: previous}))
		figure := byTestID(f, "span", "budgets-status-figure")
		if figure == nil || figure.Text() != want {
			model := computeBudgetView(app, "", window, prefs.Prefs{BudgetIncomeMode: "fixed", MonthlyIncomeMinor: 500000}, previous)
			t.Fatalf("selected window=%s previous=%v figure=%q want %s; cached summary=%d known=%v complete=%v issues=%v; diagnostics=%v", window.Label(), previous,
				budgetSummaryNativeText(f, "budgets-status-figure"), want, model.SummaryActual, model.SummaryKnown, model.SummaryComplete, model.SummaryIssues, f.BuildDiagnostics())
		}
	}
	assertFigure(window, false, "$30.00")
	assertFigure(window, true, "$20.00")
	assertFigure(window.Shift(1), false, "$0.00")
	assertFigure(window, false, "$30.00")
	budgetSummaryNativePut(t, app, "march", time.March, -4100, "USD")
	updated := computeBudgetView(app, "", window, prefs.Prefs{BudgetIncomeMode: "fixed", MonthlyIncomeMinor: 500000}, false)
	if updated.SummaryActual != 4100 || !updated.SummaryComplete {
		t.Fatalf("store revision did not invalidate the selected-window model before UI notification: %+v", updated)
	}
	// Store writes advance the cache revision but do not notify mounted UI
	// subscribers. Match external mutation controllers' normal notification.
	uistate.BumpDataRevision()
	f.Stabilize()
	assertFigure(window, false, "$41.00")
}

func TestBudgetSummaryNativeHostDisclosesPartialAndFatalSpendWithoutHidingPlan(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		fatal      bool
	}{
		{"partial FX", "$25.00", false},
		{"aggregate overflow", "—", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := budgetSummaryNativeApp(t, domain.PeriodMonthly)
			// The viewed year's monthly status evaluates January. Later rows must
			// influence the selected-year headline without contaminating that status.
			budgetSummaryNativePut(t, app, "good", time.January, -2500, "USD")
			if tc.fatal {
				budgetSummaryNativePut(t, app, "huge", time.February, -math.MaxInt64, "USD")
			} else {
				budgetSummaryNativePut(t, app, "foreign", time.February, -100, "EUR")
			}
			window := period.NewWindow(period.Year, time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC), time.Monday)
			v := computeBudgetView(app, "", window, prefs.Prefs{}, false)
			plan := budgetSummaryPlan(v, app, window)
			if !plan.normalizationOK || len(v.Incomplete) != 0 || v.SummaryComplete || v.SummaryKnown == tc.fatal {
				t.Fatalf("spending availability must be independent of valid plan/status: view=%+v plan=%+v", v, plan)
			}
			f := render.New(t)
			f.Render(ui.CreateElement(budgetSummaryNativeHost, budgetSummaryNativeProps{App: app, Window: window}))
			figure, sentence := byTestID(f, "span", "budgets-status-figure"), byTestID(f, "span", "budgets-status-sentence")
			if figure == nil || figure.Text() != tc.want || sentence == nil || !strings.Contains(sentence.Text(), "incomplete") {
				t.Fatalf("unavailable spending became a healthy headline: figure=%q sentence=%q", budgetSummaryNativeText(f, "budgets-status-figure"), budgetSummaryNativeText(f, "budgets-status-sentence"))
			}
			if byTestID(f, "div", "budgets-status-meter") != nil {
				t.Fatal("incomplete spending exposed a valid meter percentage")
			}
			unknown := false
			for _, div := range f.AllByTag("div") {
				if div.Attr("class") == "bud3-headline" && div.Attr("data-state") == "unknown" {
					unknown = true
				}
			}
			if !unknown {
				t.Fatal("incomplete spending retained healthy headline state")
			}
			if !tc.fatal {
				settings := app.Settings()
				settings.FXRates = map[string]float64{"EUR": 1}
				if err := app.PutSettings(settings); err != nil {
					t.Fatal(err)
				}
				recovered := computeBudgetView(app, "", window, prefs.Prefs{BudgetIncomeMode: "fixed", MonthlyIncomeMinor: 500000}, false)
				if recovered.SummaryActual != 2600 || !recovered.SummaryKnown || !recovered.SummaryComplete {
					t.Fatalf("FX store revision did not invalidate the model before UI notification: %+v", recovered)
				}
				// As in the settings save controller, notify mounted readers after
				// persisting the rate change; identical props alone do not rerender.
				uistate.BumpDataRevision()
				f.Stabilize()
				if got := byTestID(f, "span", "budgets-status-figure"); got == nil || got.Text() != "$26.00" {
					model := computeBudgetView(app, "", window, prefs.Prefs{BudgetIncomeMode: "fixed", MonthlyIncomeMinor: 500000}, false)
					t.Fatalf("fresh FX failed to recover selected-window spend: text=%q summary=%d known=%v complete=%v issues=%v", budgetSummaryNativeText(f, "budgets-status-figure"), model.SummaryActual, model.SummaryKnown, model.SummaryComplete, model.SummaryIssues)
				}
				if byTestID(f, "div", "budgets-status-meter") == nil {
					t.Fatal("recovered spending did not regain its meter")
				}
			}
		})
	}
}

func budgetSummaryNativeText(f *render.Fixture, id string) string {
	if node := byTestID(f, "span", id); node != nil {
		return node.Text()
	}
	return "<missing>"
}
