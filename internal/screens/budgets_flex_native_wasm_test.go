// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type flexNativeControls struct{ Window state.Atom[period.Window] }
type flexNativeProps struct {
	App      *appstate.App
	Window   period.Window
	Controls *flexNativeControls
}

func flexNativeHost(props flexNativeProps) ui.Node {
	window := uistate.UsePeriod()
	initialized := ui.UseState(false)
	props.Controls.Window = window
	if !initialized.Get() {
		initialized.Set(true)
		window.Set(props.Window)
	}
	return ui.CreateElement(budgetFlexWidget, budgetSummaryProps{App: props.App})
}

func flexNativeApp(t *testing.T) *appstate.App {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	if err := app.Store().PutCategory(domain.Category{ID: "daily", Name: "Daily", Kind: domain.KindExpense, CategoryClass: domain.ClassFlex}); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.FlexBudget = 30000
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	return app
}

func flexNativeFixture(t *testing.T) *render.Fixture {
	t.Helper()
	t.Cleanup(func() { time.Sleep(650 * time.Millisecond) })
	f := render.New(t)
	t.Cleanup(func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	})
	return f
}

func flexNativeAssertSpent(t *testing.T, f *render.Fixture, spent, target int64) {
	t.Helper()
	node := byTestID(f, "span", "flex-spent")
	want := uistate.T("flex.spentTarget", fmtMoney(money.New(spent, "USD")), fmtMoney(money.New(target, "USD")))
	if node == nil || node.Text() != want {
		t.Fatalf("flex spending/target did not use native window facts; want%q text=%s", want, f.Text())
	}
}

func TestBudgetFlexNativeHostTracksPeriodFXAndTargetEdit(t *testing.T) {
	app := flexNativeApp(t)
	for _, row := range []domain.Transaction{{ID: "april", CategoryID: "daily", Date: time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(-1000, "USD")}, {ID: "may", CategoryID: "daily", Date: time.Date(2024, 5, 10, 0, 0, 0, 0, time.UTC), Amount: money.New(-2000, "EUR")}} {
		if err := app.Store().PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	window := period.NewWindow(period.Month, time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), time.Monday)
	controls := &flexNativeControls{}
	f := flexNativeFixture(t)
	f.Render(ui.CreateElement(flexNativeHost, flexNativeProps{App: app, Window: window, Controls: controls}))
	f.Stabilize()
	flexNativeAssertSpent(t, f, 1000, 30000)
	controls.Window.Set(period.NewWindow(period.Week, time.Date(2024, 4, 8, 0, 0, 0, 0, time.UTC), time.Monday))
	f.Stabilize()
	flexNativeAssertSpent(t, f, 1000, 7000)
	controls.Window.Set(period.NewWindow(period.Quarter, time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC), time.Monday))
	f.Stabilize()
	if byTestID(f, "span", "flex-spent") != nil || !strings.Contains(f.Text(), uistate.T("flex.calculationFailed")) {
		t.Fatal("missing FX became valid flex facts", f.Text())
	}
	settings := app.Settings()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	flexNativeAssertSpent(t, f, 5000, 90000)
	byTestID(f, "button", "flex-target-edit").Click()
	f.Stabilize()
	byTestID(f, "input", "flex-target-input").Input("600.00")
	f.Stabilize()
	byTestID(f, "button", "flex-target-save").Click()
	f.Stabilize()
	flexNativeAssertSpent(t, f, 5000, 180000)
	if app.Settings().FlexBudget != 60000 {
		t.Fatal("native view adapter changed target save semantics")
	}
}

func TestBudgetFlexNativeHostUsesNativeMeterVerdictAndClockFacts(t *testing.T) {
	app := flexNativeApp(t)
	now := budgetNow()
	window := period.NewWindow(period.Month, now, time.Monday)
	if err := app.Store().PutTransaction(domain.Transaction{ID: "tight", CategoryID: "daily", Date: now, Amount: money.New(-27000, "USD")}); err != nil {
		t.Fatal(err)
	}
	controls := &flexNativeControls{}
	f := flexNativeFixture(t)
	f.Render(ui.CreateElement(flexNativeHost, flexNativeProps{App: app, Window: window, Controls: controls}))
	f.Stabilize()
	flexNativeAssertSpent(t, f, 27000, 30000)
	if !strings.Contains(f.Text(), uistate.T("flex.paceTight")) {
		t.Fatal("native tight verdict was not mapped to existing copy", f.Text())
	}
	var meterFound, tickFound bool
	for _, div := range f.AllByTag("div") {
		if div.Attr("role") == "meter" {
			meterFound = div.Attr("aria-valuenow") == "90"
		}
		if div.Attr("class") == "bflex-meter-pace" {
			tickFound = true
		}
	}
	start, end := window.Range()
	clock := budgetview.PrepareFlexMeter(budgeting.FlexView{Spent: money.New(27000, "USD"), Target: money.New(30000, "USD"), Remaining: money.New(3000, "USD")}, 30000, budgetNow(), start, end)
	if !meterFound || tickFound != clock.ShowPace {
		t.Fatalf("meter/tick wiring differs from native facts: meter%v tick%v expected%v", meterFound, tickFound, clock.ShowPace)
	}
}
