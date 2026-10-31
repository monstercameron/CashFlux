// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"io"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestDashboardPreparedFramesRefreshThroughActualHost(t *testing.T) {
	stubBudgetRowBrowser(t)
	document := js.Global().Get("document")
	previousQueryAll := document.Get("querySelectorAll")
	queryAll := js.FuncOf(func(js.Value, []js.Value) any { return []any{} })
	document.Set("querySelectorAll", queryAll)
	defer func() { document.Set("querySelectorAll", previousQueryAll); queryAll.Release() }()
	oldCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(oldCSS)
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	previousApp := appstate.Default
	appstate.Default = app
	defer func() { appstate.Default = previousApp }()
	previousClock := dashboardPreparationNow
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	dashboardPreparationNow = func() time.Time { return clock }
	defer func() { dashboardPreparationNow = previousClock }()
	oldContexts, oldLayouts, oldStored := dashboardFrameContexts, dashboardLayouts, dashboardStoredPlacements
	dashboardFrameContexts = dashboardview.NewFrameContexts(2)
	dashboardLayouts = &dashboardview.LayoutCache{}
	dashboardStoredPlacements = newViewCache[[]domain.Placement](2)
	defer func() {
		dashboardFrameContexts, dashboardLayouts, dashboardStoredPlacements = oldContexts, oldLayouts, oldStored
	}()
	var prepared *dashboardview.PreparedFrames
	var recent, cashflow domain.Frame
	rendered := make(chan struct{}, 1)
	widgetrender.RegisterFrame("recent", func(frame domain.Frame, ctx widgetrender.RenderCtx) ui.Node {
		prepared, recent = ctx.Frames, frame
		select {
		case rendered <- struct{}{}:
		default:
		}
		return recentFrame(frame, ctx)
	})
	defer widgetrender.RegisterFrame("recent", recentFrame)
	widgetrender.RegisterFrame("cashflow", func(frame domain.Frame, ctx widgetrender.RenderCtx) ui.Node {
		cashflow = frame
		return cashFlowFrame(frame, ctx)
	})
	defer widgetrender.RegisterFrame("cashflow", cashFlowFrame)
	f := render.New(t)
	defer f.Cleanup()
	var items state.Atom[[]dashlayout.Item]
	var configs state.Atom[uistate.WidgetConfigs]
	var hidden state.Atom[widgetvis.Set]
	var lens state.Atom[scope.ReportScope]
	var window state.Atom[period.Window]
	var editing state.Atom[bool]
	var mount func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		items, configs, hidden = uistate.UseLayoutItems(), uistate.UseWidgetConfigs(), uistate.UseHiddenWidgets()
		lens, window, editing = uistate.UseActiveScope(), uistate.UsePeriod(), uistate.UseLayoutEdit()
		ready := ui.UseState(false)
		mount = ready.Set
		if ready.Get() {
			return ui.CreateElement(Dashboard)
		}
		return nil
	}))
	oldItems, oldConfigs, oldHidden, oldLens, oldWindow, oldEditing := items.Get(), configs.Get(), hidden.Get(), lens.Get(), window.Get(), editing.Get()
	defer func() {
		mount(false)
		items.Set(oldItems)
		configs.Set(oldConfigs)
		hidden.Set(oldHidden)
		lens.Set(oldLens)
		window.Set(oldWindow)
		editing.Set(oldEditing)
	}()
	items.Set([]dashlayout.Item{{ID: "recent", ColSpan: 2, RowSpan: 2}, {ID: "cashflow", ColSpan: 2, RowSpan: 2}})
	configs.Set(uistate.WidgetConfigs{"recent": {"display": "cap", "count": "6"}})
	hidden.Set(widgetvis.Set{})
	lens.Set(scope.ReportScope{})
	window.Set(period.NewWindow(period.Month, clock, time.Monday))
	editing.Set(false)
	mount(true)
	select {
	case <-rendered:
	case <-time.After(3 * time.Second):
		t.Fatal("Dashboard never prepared its deferred frames")
	}
	f.Stabilize()
	if prepared == nil || recent.Rows != 0 {
		t.Fatal("empty Dashboard did not use shared empty snapshot")
	}
	for _, account := range []domain.Account{
		{ID: "a", Name: "Cash", Institution: "A B", Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking},
		{ID: "b", Name: "Euro", Institution: "A", Currency: "EUR", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking},
		{ID: "c", Name: "Other", Institution: "C", Currency: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking},
	} {
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"a", "b"} {
		currency := "USD"
		if id == "b" {
			currency = "EUR"
		}
		if err := app.PutTransaction(domain.Transaction{ID: "expense-" + id, AccountID: id, Desc: "Loaded " + id, Date: clock.AddDate(0, 0, -i-1), Amount: money.New(-10000, currency)}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := app.PutTransaction(domain.Transaction{ID: fmt.Sprintf("extra-%d", i), AccountID: "c", Desc: "Additional income", Date: clock.AddDate(0, 0, -5-i), Amount: money.New(1000, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	empty := prepared
	uistate.BumpDataRevision()
	f.Stabilize()
	if prepared == empty || recent.Rows != 4 || !strings.Contains(f.Text(), "Loaded a") {
		t.Fatal("loaded ledger reused empty frames")
	}
	// Placement persistence changes the broad app revision once. Absorb that
	// write, then use an unrelated real host atom to rerender without data edits.
	editing.Set(true)
	f.Stabilize()
	editing.Set(false)
	f.Stabilize()
	warmed, hydrations, layoutBuilds := prepared, prepared.Hydrations(), dashboardLayouts.Builds()
	for i := 0; i < 4; i++ {
		editing.Set(!editing.Get())
		f.Stabilize()
	}
	if prepared != warmed || prepared.Hydrations() != hydrations {
		t.Fatalf("unchanged parent renders rehydrated frames: old=%d now=%d", hydrations, prepared.Hydrations())
	}
	if dashboardLayouts.Builds() != layoutBuilds {
		t.Fatal("unchanged host renders rebuilt/serialized layout")
	}
	// A return visit remounts the controller. Its effect must not rewrite the
	// same placements and invalidate the prepared financial snapshot.
	mount(false)
	f.Stabilize()
	beforeRemount := app.Rev()
	prepared = nil
	rendered = make(chan struct{}, 1)
	mount(true)
	select {
	case <-rendered:
	case <-time.After(3 * time.Second):
		t.Fatal("remounted Dashboard never rendered its deferred grid")
	}
	f.Stabilize()
	if app.Rev() != beforeRemount {
		t.Fatalf("unchanged remount advanced app revision: %d → %d", beforeRemount, app.Rev())
	}
	if prepared != warmed || prepared.Hydrations() != hydrations {
		t.Fatal("unchanged remount discarded prepared financial frames")
	}
	configs.Set(configs.Get().WithField("recent", "count", "3"))
	f.Stabilize()
	if recent.Rows != 3 {
		t.Fatalf("settings change retained %d recent rows", recent.Rows)
	}
	configs.Set(configs.Get().WithField("recent", "count", "6"))
	f.Stabilize()
	lens.Set(scope.ReportScope{Institutions: []string{"A B"}})
	f.Stabilize()
	desc, _ := recent.Column("desc")
	if recent.Rows != 1 || desc.Str(0) != "Loaded a" {
		t.Fatal("scope retained household recent rows")
	}
	lens.Set(scope.ReportScope{Institutions: []string{"A", "B"}})
	f.Stabilize()
	desc, _ = recent.Column("desc")
	if recent.Rows != 1 || desc.Str(0) != "Loaded b" {
		t.Fatal("ambiguous scope signature retained another institution")
	}
	lens.Set(scope.ReportScope{})
	f.Stabilize()
	expense, _ := cashflow.Column("expense")
	if expense.Int64(len(expense.Values)-1) != 30000 {
		t.Fatalf("FX cashflow got %+v", expense.Values)
	}
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	expense, _ = cashflow.Column("expense")
	if expense.Int64(len(expense.Values)-1) != 40000 {
		t.Fatal("FX revision retained old frame values")
	}
	window.Set(period.NewWindow(period.Month, clock.AddDate(0, -1, 0), time.Monday))
	f.Stabilize()
	times, _ := cashflow.Column("t")
	if time.Unix(int64(times.Num(len(times.Values)-1)), 0).UTC().Month() != time.September {
		t.Fatal("period change retained current-month chart")
	}
	beforeClock := prepared
	clock = clock.Add(time.Minute)
	editing.Set(!editing.Get())
	f.Stabilize()
	if prepared == beforeClock {
		t.Fatal("clock bucket retained prior current-state snapshot")
	}
	// The same mounted Dashboard can receive another App with the same layout.
	// Its effect must sync that app independently even when the signature matches.
	other, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Store().Close()
	appstate.Default = other
	uistate.BumpDataRevision()
	f.Stabilize()
	if got := other.Placements("dashboard"); len(got) != 3 {
		t.Fatalf("replacement app stored %d Dashboard placements, want 3", len(got))
	}
	appstate.Default = app
	uistate.BumpDataRevision()
	f.Stabilize()
	appstate.Default = nil
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(f.Text(), uistate.T("common.notReady")) {
		t.Fatal("unavailable app retained loaded Dashboard")
	}
	appstate.Default = app
}

func TestDashboardEffectivePipelineDoesNotMutateCachedSpec(t *testing.T) {
	transforms := make([]domain.Transform, 1, 4)
	transforms[0] = domain.Transform{Kind: domain.TransformSort, Arg: "-date"}
	backing := transforms[:cap(transforms)]
	backing[1] = domain.Transform{Kind: domain.TransformFilter, Arg: "sentinel"}
	spec := domain.WidgetSpec{ID: "recent", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions"}, Transform: transforms}, Settings: map[string]string{"display": "cap", "count": "3"}}
	pipe := effectivePipeline(spec, widgetrender.RenderCtx{})
	if len(pipe.Transform) != 2 || pipe.Transform[1].N != 3 || backing[1].Arg != "sentinel" {
		t.Fatal("effective pipeline appended into immutable spec backing storage")
	}
}
