// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestPrimaryNetWorthDashboardKpiAndHouseholdDisclosure(t *testing.T) {
	stubBudgetRowBrowser(t)
	doc := js.Global().Get("document")
	previousQuery := doc.Get("querySelectorAll")
	query := js.FuncOf(func(js.Value, []js.Value) any { return []any{} })
	doc.Set("querySelectorAll", query)
	defer func() { doc.Set("querySelectorAll", previousQuery); query.Release() }()
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
	ready := make(chan struct{}, 1)
	widgetrender.RegisterFrame("recent", func(frame domain.Frame, ctx widgetrender.RenderCtx) ui.Node {
		select {
		case ready <- struct{}{}:
		default:
		}
		return recentFrame(frame, ctx)
	})
	defer widgetrender.RegisterFrame("recent", recentFrame)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	var items state.Atom[[]dashlayout.Item]
	var configs state.Atom[uistate.WidgetConfigs]
	var hidden state.Atom[widgetvis.Set]
	var lens state.Atom[scope.ReportScope]
	var mount func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		items, configs, hidden = uistate.UseLayoutItems(), uistate.UseWidgetConfigs(), uistate.UseHiddenWidgets()
		lens = uistate.UseActiveScope()
		mounted := ui.UseState(false)
		mount = mounted.Set
		if mounted.Get() {
			return ui.CreateElement(Dashboard)
		}
		return nil
	}))
	oldItems, oldConfigs, oldHidden, oldLens := items.Get(), configs.Get(), hidden.Get(), lens.Get()
	defer func() {
		mount(false)
		items.Set(oldItems)
		configs.Set(oldConfigs)
		hidden.Set(oldHidden)
		lens.Set(oldLens)
	}()
	items.Set([]dashlayout.Item{{ID: "kpi-networth", ColSpan: 1, RowSpan: 1}, {ID: "recent", ColSpan: 2, RowSpan: 2}})
	configs.Set(uistate.WidgetConfigs{})
	hidden.Set(widgetvis.Set{})
	lens.Set(scope.ReportScope{})
	mount(true)
	deadline := time.Now().Add(3 * time.Second)
	for len(ready) == 0 && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(10 * time.Millisecond)
	}
	if len(ready) == 0 {
		t.Fatal("Dashboard did not finish deferred KPI render")
	}
	f.Stabilize()
	kpi := func() string {
		t.Helper()
		for _, node := range f.AllByTag("div") {
			if node.Attr("data-widget") == "kpi-networth" {
				return node.Text()
			}
		}
		t.Fatal("missing actual Dashboard net-worth KPI")
		return ""
	}
	if text := kpi(); !strings.Contains(text, "100.00") || !strings.Contains(text, "Partial total") || !strings.Contains(text, "EUR") {
		t.Fatalf("KPI hid missing FX: %s", text)
	}
	debt.Currency, debt.OpeningBalance = "GBP", money.New(-1000, "GBP")
	if err := app.PutAccount(debt); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	// A balance with no exchange rate is left out of the total and named, never summed in.
	if text := kpi(); !strings.Contains(text, "100.00") || !strings.Contains(text, "Partial total") || !strings.Contains(text, "GBP") {
		t.Fatalf("KPI did not leave the unpriced balance out of the total: %s", text)
	}
	lens.Set(scope.ReportScope{AccountIDs: []string{bank.ID}})
	f.Stabilize()
	// The bank-only scope has no unpriced account, so its figure stands on its own.
	if text := kpi(); !strings.Contains(text, "100.00") {
		t.Fatalf("scoped KPI lost its figure: %s", text)
	}
}
