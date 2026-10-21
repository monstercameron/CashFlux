// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"log/slog"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/periodview"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
	"github.com/monstercameron/GoWebComponents/v5/css"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestDashboardPeriodTotalsUnknownScopeAndFXRecovery(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(previousLog)
	doc := js.Global().Get("document")
	previousQuery := doc.Get("querySelectorAll")
	query := js.FuncOf(func(js.Value, []js.Value) any { return []any{} })
	doc.Set("querySelectorAll", query)
	defer func() { doc.Set("querySelectorAll", previousQuery); query.Release() }()
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := agrApp(t)
	defer app.Store().Close()
	if err := app.PutAccount(domain.Account{ID: "foreign", Name: "Foreign", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "foreign-expense", AccountID: "foreign", Desc: "Foreign purchase", Date: agrDay, Amount: money.New(-1000, "EUR")}); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	var observed periodview.Totals
	ready := make(chan struct{}, 1)
	widgetrender.RegisterFrame("recent", func(frame domain.Frame, ctx widgetrender.RenderCtx) ui.Node {
		observed = ctx.PeriodTotals
		select {
		case ready <- struct{}{}:
		default:
		}
		return recentFrame(frame, ctx)
	})
	defer widgetrender.RegisterFrame("recent", recentFrame)
	f := render.New(t)
	defer f.Cleanup()
	var items state.Atom[[]dashlayout.Item]
	var configs state.Atom[uistate.WidgetConfigs]
	var hidden state.Atom[widgetvis.Set]
	var lens state.Atom[scope.ReportScope]
	var window state.Atom[period.Window]
	var mount func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		items, configs, hidden = uistate.UseLayoutItems(), uistate.UseWidgetConfigs(), uistate.UseHiddenWidgets()
		lens, window = uistate.UseActiveScope(), uistate.UsePeriod()
		mounted := ui.UseState(false)
		mount = mounted.Set
		if mounted.Get() {
			return ui.CreateElement(Dashboard)
		}
		return nil
	}))
	oldItems, oldConfigs, oldHidden, oldLens, oldWindow := items.Get(), configs.Get(), hidden.Get(), lens.Get(), window.Get()
	defer func() {
		mount(false)
		items.Set(oldItems)
		configs.Set(oldConfigs)
		hidden.Set(oldHidden)
		lens.Set(oldLens)
		window.Set(oldWindow)
	}()
	items.Set([]dashlayout.Item{{ID: "kpi-income", ColSpan: 1, RowSpan: 1}, {ID: "savings", ColSpan: 2, RowSpan: 1}, {ID: "recent", ColSpan: 2, RowSpan: 2}})
	configs.Set(uistate.WidgetConfigs{})
	hidden.Set(widgetvis.Set{})
	lens.Set(scope.ReportScope{})
	window.Set(period.NewWindow(period.Month, agrDay, time.Monday))
	mount(true)
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("Dashboard did not finish deferred KPI render")
	}
	f.Stabilize()
	widgetText := func(id string) string {
		for _, node := range f.AllByTag("div") {
			if node.Attr("data-widget") == id {
				return node.Text()
			}
		}
		return ""
	}
	assertUnknown := func() {
		t.Helper()
		if observed.Known || observed.Error == nil {
			t.Fatalf("context exposed unavailable totals as valid: %+v", observed)
		}
		if byTestID(f, "p", "hero-totals-unavailable") == nil || byTestID(f, "div", "savings-unavailable") == nil {
			t.Fatalf("missing unknown disclosure: %s", f.Text())
		}
		if text := widgetText("kpi-income"); !strings.Contains(text, "—") {
			t.Fatalf("income looked valid: %q", text)
		}
	}
	assertUnknown()
	// A valid scope recovers independently of missing household FX.
	lens.Set(scope.ReportScope{AccountIDs: []string{"a-check"}})
	f.Stabilize()
	if !observed.Known || observed.Income.Amount != 450000 || byTestID(f, "p", "hero-totals-unavailable") != nil || byTestID(f, "div", "savings-unavailable") != nil {
		t.Fatalf("scope recovery: %+v %s", observed, f.Text())
	}
	lens.Set(scope.ReportScope{})
	f.Stabilize()
	assertUnknown()
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !observed.Known || observed.Net.Amount != 275500 || !strings.Contains(widgetText("savings"), "61%") || byTestID(f, "p", "hero-totals-unavailable") != nil {
		t.Fatalf("FX recovery: %+v %s", observed, f.Text())
	}
	// An empty valid period remains known zero rather than an error disclosure.
	window.Set(period.NewWindow(period.Month, agrDay.AddDate(0, 1, 0), time.Monday))
	f.Stabilize()
	if !observed.Known || observed.Income.Amount != 0 || observed.Expense.Amount != 0 || byTestID(f, "p", "hero-totals-unavailable") != nil {
		t.Fatalf("empty period: %+v %s", observed, f.Text())
	}
}

func TestAssistantAffordabilityWaitsForKnownPeriodTotals(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer slog.SetDefault(previousLog)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	app := agrApp(t)
	defer app.Store().Close()
	now := time.Now()
	for _, txn := range app.Transactions() {
		txn.Date = now
		if err := app.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.PutAccount(domain.Account{ID: "foreign", Name: "Foreign", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "foreign-expense", AccountID: "foreign", Desc: "Foreign purchase", Date: now, Amount: money.New(-1000, "EUR")}); err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	// The production composer reads its DOM value directly to avoid rerenders on
	// each keystroke. Supply that small browser boundary and click its real button.
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
	defer func() { time.Sleep(120 * time.Millisecond); doc.Set("getElementById", previousGet); get.Release() }()
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Insights))
	f.Stabilize()
	if byTestID(f, "p", "assistant-totals-unavailable") == nil {
		t.Fatal("missing Assistant totals disclosure")
	}
	send := func() {
		t.Helper()
		composer.Set("value", "Can I afford $500?")
		button := byTestID(f, "button", "assistant-send")
		if button == nil {
			t.Fatal("missing real send button")
		}
		button.Click()
		f.Stabilize()
	}
	affordCount := func() int {
		n := 0
		for _, node := range f.AllByTag("div") {
			if node.Attr("data-cf") == "afford-result" {
				n++
			}
		}
		return n
	}
	send()
	renderedUnavailable := false
	for _, el := range markdown {
		if strings.Contains(el.Get("innerHTML").String(), uistate.T("insights.affordTotalsUnavailable")) {
			renderedUnavailable = true
		}
	}
	if !renderedUnavailable || affordCount() != 0 {
		t.Fatalf("unknown totals produced recommendation: %s", f.Text())
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "assistant-totals-unavailable") != nil {
		t.Fatal("Assistant retained missing FX disclosure")
	}
	send()
	if affordCount() != 1 {
		t.Fatalf("known totals did not recover deterministic affordability card: %s", f.Text())
	}
}

func TestSavingsGaugeRetainsWideSignedPercentage(t *testing.T) {
	stubBudgetRowBrowser(t)
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	f := render.New(t)
	defer f.Cleanup()
	var setRate func(float64)
	f.Render(ui.CreateElement(func() ui.Node {
		rate := ui.UseState(float64(-2999999900))
		setRate = rate.Set
		return renderKPISpec(domain.WidgetSpec{ID: "savings", Settings: widgetcfg.Config{"formula": "savings_rate"}, Scalar: &domain.ScalarBind{Expr: "savings_rate", Format: widgetspec.FormatPercent}}, widgetrender.RenderCtx{Vars: map[string]float64{"savings_rate": rate.Get()}})
	}))
	if !strings.Contains(f.Text(), "-2999999900%") {
		t.Fatalf("gauge narrowed percentage: %s", f.Text())
	}
	setRate(-0.3)
	f.Stabilize()
	if strings.Contains(f.Text(), "-0%") {
		t.Fatalf("gauge changed truncation at negative zero: %s", f.Text())
	}
}
