// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func widgetInitialTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous; _ = app.Store().Close() })
	return app
}

type widgetInitialReaderProps struct {
	Name string
	Tick int
}

func TestWidgetLayoutInitializersRunOncePerMountedHook(t *testing.T) {
	widgetInitialTestApp(t)
	PersistDashPreset("daily")
	PersistItems([]dashlayout.Item{{ID: "recent", ColSpan: 3, RowSpan: 2}})
	PersistWidgetConfigs(WidgetConfigs{"recent": {"count": "4"}})
	layoutLoads, configLoads := map[string]int{}, map[string]int{}
	layouts := map[string]state.Atom[[]dashlayout.Item]{}
	configs := map[string]state.Atom[WidgetConfigs]{}
	reader := func(p widgetInitialReaderProps) ui.Node {
		layout := useLayoutItemsWithLoader(func() []dashlayout.Item { layoutLoads[p.Name]++; return loadItems() })
		config := useWidgetConfigsWithLoader(func() WidgetConfigs { configLoads[p.Name]++; return loadWidgetConfigs() })
		layouts[p.Name], configs[p.Name] = layout, config
		items := layout.Get()
		first := "empty"
		span := 0
		if len(items) > 0 {
			first, span = items[0].ID, items[0].ColSpan
		}
		return P(Attr("id", p.Name), fmt.Sprintf("%s:%d/%s", first, span, config.Get().For("recent")["count"]))
	}
	f := render.New(t)
	defer f.Cleanup()
	var setTick func(int)
	var setSecond, setLate func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		tick := ui.UseState(0)
		second := ui.UseState(true)
		late := ui.UseState(false)
		setTick, setSecond, setLate = tick.Set, second.Set, late.Set
		var peer, extra ui.Node
		if second.Get() {
			peer = ui.CreateElement(reader, widgetInitialReaderProps{Name: "second", Tick: tick.Get()})
		}
		if late.Get() {
			extra = ui.CreateElement(reader, widgetInitialReaderProps{Name: "late", Tick: tick.Get()})
		}
		return Div(ui.CreateElement(reader, widgetInitialReaderProps{Name: "first", Tick: tick.Get()}), Div(peer), Div(extra))
	}))
	assertReader := func(id, want string, loads int) {
		t.Helper()
		node := f.ByID(id)
		if layoutLoads[id] != loads || configLoads[id] != loads || node == nil || node.Text() != want {
			t.Fatalf("reader %s: layout loads=%v config loads=%v text=%s want=%s", id, layoutLoads, configLoads, f.Text(), want)
		}
	}
	assertReader("first", "recent:3/4", 1)
	assertReader("second", "recent:3/4", 1)
	for tick := 1; tick <= 3; tick++ {
		setTick(tick)
		f.Stabilize()
	}
	assertReader("first", "recent:3/4", 1)
	assertReader("second", "recent:3/4", 1)
	// Live layout/config changes remain authoritative even before persistence.
	layouts["first"].Set([]dashlayout.Item{{ID: "wb:live", ColSpan: 2, RowSpan: 1}})
	configs["first"].Set(configs["first"].Get().WithField("recent", "count", "7"))
	f.Stabilize()
	assertReader("first", "wb:live:2/7", 1)
	assertReader("second", "wb:live:2/7", 1)
	// Change persisted defaults independently. Late/new readers load those seeds
	// once but must still observe the existing live atoms rather than overwrite.
	PersistItems([]dashlayout.Item{{ID: "budgets", ColSpan: 1, RowSpan: 2}})
	PersistWidgetConfigs(WidgetConfigs{"recent": {"count": "42"}})
	setLate(true)
	f.Stabilize()
	assertReader("late", "wb:live:2/7", 1)
	assertReader("first", "wb:live:2/7", 1)
	setSecond(false)
	f.Stabilize()
	setSecond(true)
	f.Stabilize()
	assertReader("second", "wb:live:2/7", 2)
	assertReader("first", "wb:live:2/7", 1)
}

func TestWidgetLayoutInitializersResetWithWorkspaceRuntime(t *testing.T) {
	first := widgetInitialTestApp(t)
	PersistDashPreset("daily")
	PersistItems([]dashlayout.Item{{ID: "recent", ColSpan: 3, RowSpan: 2}})
	PersistWidgetConfigs(WidgetConfigs{"recent": {"count": "4"}})
	second, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Store().Close()
	appstate.Default = second
	PersistDashPreset("payday")
	PersistItems([]dashlayout.Item{{ID: "budgets", ColSpan: 1, RowSpan: 2}})
	PersistWidgetConfigs(WidgetConfigs{"recent": {"count": "9"}})
	for _, tc := range []struct {
		app  *appstate.App
		want string
	}{{first, "recent:3/4"}, {second, "budgets:1/9"}, {first, "recent:3/4"}} {
		appstate.Default = tc.app
		// New resets the framework runtime/atom registry, matching workspace reload.
		f := render.New(t)
		f.Render(ui.CreateElement(func() ui.Node {
			items, config := UseLayoutItems().Get(), UseWidgetConfigs().Get()
			return P(fmt.Sprintf("%s:%d/%s", items[0].ID, items[0].ColSpan, config.For("recent")["count"]))
		}))
		if got := f.Text(); got != tc.want {
			f.Cleanup()
			t.Fatalf("workspace default leaked: got=%q want=%q", got, tc.want)
		}
		f.Cleanup()
	}
}

func TestLayoutInitializerPreservesSavedFormatsAndFocusPreset(t *testing.T) {
	widgetInitialTestApp(t)
	legacy := `[{"id":"recent","col":3,"row":7,"colSpan":3,"rowSpan":2},{"id":"wb:custom","col":1,"row":1,"colSpan":1,"rowSpan":1},{"id":"retired-widget","colSpan":1,"rowSpan":1}]`
	saved := []dashlayout.Item{{ID: "recent", ColSpan: 3, RowSpan: 2}, {ID: "wb:custom", ColSpan: 1, RowSpan: 1}, {ID: "retired-widget", ColSpan: 1, RowSpan: 1}}
	for _, tc := range []struct {
		name, raw, preset string
		want              []dashlayout.Item
	}{
		{name: "absent", want: dashlayout.DefaultLayoutItems()},
		{name: "invalid", raw: "{", want: dashlayout.DefaultLayoutItems()},
		{name: "empty focus still defaults", raw: "[]", preset: "daily", want: dashlayout.DefaultLayoutItems()},
		{name: "missing first ID", raw: `[{"colSpan":2}]`, want: dashlayout.DefaultLayoutItems()},
		{name: "legacy reconciliation", raw: legacy, want: dashlayout.Reconcile(saved)},
		{name: "default preset reconciles", raw: legacy, preset: "default", want: dashlayout.Reconcile(saved)},
		{name: "focus subset is authoritative", raw: legacy, preset: "daily", want: saved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kvSet(layoutStoreID, tc.raw)
			PersistDashPreset(tc.preset)
			f := render.New(t)
			defer f.Cleanup()
			var got []dashlayout.Item
			f.Render(ui.CreateElement(func() ui.Node { got = UseLayoutItems().Get(); return nil }))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("layout load changed: got=%+v want=%+v", got, tc.want)
			}
		})
	}
}

func TestWidgetConfigInitializerPreservesFallbackAndSavedSettings(t *testing.T) {
	widgetInitialTestApp(t)
	for _, tc := range []struct {
		name, raw string
		want      WidgetConfigs
	}{
		{name: "absent", want: WidgetConfigs{}},
		{name: "invalid", raw: "{", want: WidgetConfigs{}},
		{name: "saved settings", raw: `{"recent":{"count":"12","display":"cap"},"wb:custom":{"formula":"income-expense"}}`, want: WidgetConfigs{"recent": {"count": "12", "display": "cap"}, "wb:custom": {"formula": "income-expense"}}},
		{name: "null preserves optional config", raw: "null", want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kvSet(widgetCfgStoreID, tc.raw)
			f := render.New(t)
			defer f.Cleanup()
			var got WidgetConfigs
			f.Render(ui.CreateElement(func() ui.Node { got = UseWidgetConfigs().Get(); return nil }))
			if !reflect.DeepEqual(got, tc.want) || got.For("missing") == nil {
				t.Fatalf("widget settings load changed: got=%+v want=%+v", got, tc.want)
			}
		})
	}
}
