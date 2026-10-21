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
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func autoBudgetTuningHost(window period.Window) ui.Node {
	periodAtom := uistate.UsePeriod()
	current := periodAtom.Get()
	if !current.From.Equal(window.From) || !current.To.Equal(window.To) {
		periodAtom.Set(window)
	}
	prefAtom := uistate.UsePrefs()
	if pr := prefAtom.Get(); pr.BudgetIncomeMode != "fixed" || pr.MonthlyIncomeMinor != 500000 {
		prefAtom.Set(prefs.Prefs{BudgetIncomeMode: "fixed", MonthlyIncomeMinor: 500000})
	}
	return ui.CreateElement(AutoBudgetBody, struct{}{})
}

func autoBudgetTuningApp(t *testing.T, history map[string][]int64) (*appstate.App, period.Window) {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	month := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	for category, amounts := range history {
		if err := app.Store().PutCategory(domain.Category{ID: category, Name: category, Kind: domain.KindExpense}); err != nil {
			t.Fatal(err)
		}
		for i, amount := range amounts {
			row := domain.Transaction{ID: fmt.Sprintf("%s-%d", category, i), CategoryID: category, Date: month.AddDate(0, -i-1, 9), Amount: money.New(-amount, "USD")}
			if err := app.Store().PutTransaction(row); err != nil {
				t.Fatal(err)
			}
		}
	}
	return app, period.NewWindow(period.Month, month, time.Monday)
}

func autoBudgetTuningNode(t *testing.T, f *render.Fixture, tag, id string) *render.QueryNode {
	t.Helper()
	if node := byTestID(f, tag, id); node != nil {
		return node
	}
	t.Fatalf("missing %s %s; text=%s diagnostics=%v", tag, id, f.Text(), f.BuildDiagnostics())
	return nil
}

func autoBudgetTuningFixture(t *testing.T) *render.Fixture {
	t.Helper()
	// Cleanup callbacks run in reverse order: unmount the host, drain the
	// scheduled field/render work, then let the app fixture restore Default.
	t.Cleanup(func() { time.Sleep(650 * time.Millisecond) })
	f := render.New(t)
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	})
	return f
}

func TestAutoBudgetTuningHostSharesAmountsAcrossControlsImpactAndSave(t *testing.T) {
	app, window := autoBudgetTuningApp(t, map[string][]int64{"food": {10000, 30000, 20000}, "home": {8000, 8000, 8000}})
	f := autoBudgetTuningFixture(t)
	f.Render(ui.CreateElement(autoBudgetTuningHost, window))
	f.Stabilize()
	assertAmounts := func(food, total string) {
		t.Helper()
		if got := autoBudgetTuningNode(t, f, "span", "autobudget-amt-food").Text(); got != food+"/mo" {
			t.Fatalf("food row=%q want%s/mo", got, food)
		}
		if got := autoBudgetTuningNode(t, f, "span", "autobudget-total").Text(); got != "Total "+total+"/mo" {
			t.Fatalf("footer=%q want Total %s/mo", got, total)
		}
		if got := autoBudgetTuningNode(t, f, "span", "autobudget-impact-after").Text(); got != total {
			t.Fatalf("impact=%q differs from selected total %s", got, total)
		}
	}
	assertAmounts("$200.00", "$280.00")
	autoBudgetTuningNode(t, f, "input", "autobudget-slider-food").Input("150")
	f.Stabilize()
	assertAmounts("$300.00", "$380.00")
	autoBudgetTuningNode(t, f, "input", "autobudget-pick-home").Change("false")
	f.Stabilize()
	assertAmounts("$300.00", "$300.00")
	autoBudgetTuningNode(t, f, "button", "autobudget-method-healthy").Click()
	f.Stabilize()
	assertAmounts("$225.00", "$225.00")
	if got := autoBudgetTuningNode(t, f, "input", "autobudget-slider-food").Property("value"); fmt.Sprint(got) != "150" {
		t.Fatalf("method switch lost keyed food tuning: %v", got)
	}
	autoBudgetTuningNode(t, f, "button", "autobudget-reset").Click()
	f.Stabilize()
	assertAmounts("$150.00", "$150.00")
	autoBudgetTuningNode(t, f, "button", "autobudget-all").Click()
	f.Stabilize()
	assertAmounts("$150.00", "$230.00")
	autoBudgetTuningNode(t, f, "button", "autobudget-save").Click()
	f.Stabilize()
	budgets := app.Budgets()
	if len(budgets) != 2 {
		t.Fatalf("save wrote %d budgets, want2: %+v", len(budgets), budgets)
	}
	for _, budget := range budgets {
		want := int64(15000)
		if budget.CategoryID == "home" {
			want = 8000
		}
		if budget.Period != domain.PeriodMonthly || budget.Limit != money.New(want, "USD") {
			t.Fatalf("saved limit differs from prepared row/impact: %+v", budget)
		}
	}
}

func TestAutoBudgetTuningHostBlocksOverflowAndRecoversWithoutRemount(t *testing.T) {
	for _, totalFailure := range []bool{false, true} {
		name := "row scaling"
		history := map[string][]int64{"large": {math.MaxInt64}}
		if totalFailure {
			name = "selected total"
			history = map[string][]int64{"large": {math.MaxInt64/2 + 1}, "other": {math.MaxInt64/2 + 1}}
		}
		t.Run(name, func(t *testing.T) {
			app, window := autoBudgetTuningApp(t, history)
			f := autoBudgetTuningFixture(t)
			f.Render(ui.CreateElement(autoBudgetTuningHost, window))
			f.Stabilize()
			if !totalFailure {
				// Identity scaling must preserve the full valid int64 amount even
				// though its intermediate multiplication by 100 exceeds int64.
				want := fmtMoney(money.New(math.MaxInt64, "USD")) + "/mo"
				if got := autoBudgetTuningNode(t, f, "span", "autobudget-amt-large").Text(); got != want {
					t.Fatalf("valid large identity scaling=%q want%q", got, want)
				}
				autoBudgetTuningNode(t, f, "input", "autobudget-slider-large").Input("200")
				f.Stabilize()
				if got := autoBudgetTuningNode(t, f, "span", "autobudget-amt-large").Text(); !strings.Contains(got, uistate.T("calculation.unavailable")) {
					t.Fatalf("overflowed row exposed a numeric amount: %q", got)
				}
			}
			if got := autoBudgetTuningNode(t, f, "span", "autobudget-total").Text(); got != "Total —/mo" {
				t.Fatalf("unavailable total became plausible money: %q", got)
			}
			autoBudgetTuningNode(t, f, "p", "autobudget-tuning-error")
			if byTestID(f, "div", "autobudget-impact") != nil {
				t.Fatal("invalid tuning exposed an affordability impact")
			}
			save := autoBudgetTuningNode(t, f, "button", "autobudget-save")
			if !isDisabled(save) {
				t.Fatalf("invalid selection left Save enabled: property=%v attr=%q", save.Property("disabled"), save.Attr("disabled"))
			}
			save.Click()
			f.Stabilize()
			if got := app.Budgets(); len(got) != 0 {
				t.Fatalf("invalid tuning wrote budgets: %+v", got)
			}
			if totalFailure {
				autoBudgetTuningNode(t, f, "input", "autobudget-pick-other").Change("false")
			} else {
				autoBudgetTuningNode(t, f, "input", "autobudget-slider-large").Input("100")
			}
			f.Stabilize()
			if byTestID(f, "p", "autobudget-tuning-error") != nil {
				t.Fatal("tuning error did not recover after valid edit")
			}
			if disabled, _ := autoBudgetTuningNode(t, f, "button", "autobudget-save").Property("disabled").(bool); disabled {
				t.Fatal("recovered valid tuning stayed disabled")
			}
			if byTestID(f, "div", "autobudget-impact") == nil {
				t.Fatal("valid recovered selection did not regain its impact")
			}
		})
	}
}
