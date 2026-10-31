// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func txFilterInitialApp(t *testing.T) *appstate.App {
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

type txFilterInitialReaderProps struct {
	Name string
	Tick int
}

func TestTxFilterInitialLoadRunsOncePerMountedHook(t *testing.T) {
	app := txFilterInitialApp(t)
	stored := (TxFilter{Text: "Stored", Account: "bank", From: "2026-01-02", To: "2026-01-20", PeriodPreset: txnfilter.PeriodCustom, Sort: "amount", Dir: txnfilter.Asc}).Normalize()
	PersistTxFilter(stored)
	loads := map[string]int{}
	atoms := map[string]state.Atom[TxFilter]{}
	reader := func(p txFilterInitialReaderProps) ui.Node {
		a := useTxFilterWithLoader(func() TxFilter { loads[p.Name]++; return loadTxFilter() })
		atoms[p.Name] = a
		return P(Attr("id", p.Name), a.Get().Text)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	var setTick func(int)
	var setSecond, setLate func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		tick, second, late := ui.UseState(0), ui.UseState(true), ui.UseState(false)
		setTick, setSecond, setLate = tick.Set, second.Set, late.Set
		var secondNode, lateNode ui.Node
		if second.Get() {
			secondNode = ui.CreateElement(reader, txFilterInitialReaderProps{Name: "second", Tick: tick.Get()})
		}
		if late.Get() {
			lateNode = ui.CreateElement(reader, txFilterInitialReaderProps{Name: "late", Tick: tick.Get()})
		}
		return Div(ui.CreateElement(reader, txFilterInitialReaderProps{Name: "first", Tick: tick.Get()}), Div(secondNode), Div(lateNode))
	}))
	if loads["first"] != 1 || loads["second"] != 1 || atoms["first"].Get() != stored {
		t.Fatalf("initial filter loads=%v value=%+v", loads, atoms["first"].Get())
	}
	for tick := 1; tick <= 3; tick++ {
		setTick(tick)
		f.Stabilize()
	}
	changed := stored
	changed.Text, changed.Sort, changed.Dir = "Latest", "payee", txnfilter.Desc
	PersistTxFilter(changed)
	atoms["first"].Set(changed)
	f.Stabilize()
	if atoms["second"].Get() != changed || f.ByID("second").Text() != "Latest" {
		t.Fatal("filter subscribers lost a live persisted write")
	}
	if err := app.SetKV("tx-filter-test:unrelated", "revision"); err != nil {
		t.Fatal(err)
	}
	setTick(4)
	f.Stabilize()
	if loads["first"] != 1 || loads["second"] != 1 {
		t.Fatalf("warm renders or store revisions reread ignored defaults: %v", loads)
	}
	// The live atom can intentionally be ahead of persistence (debounced search).
	live := changed
	live.Text = "Live only"
	atoms["first"].Set(live)
	f.Stabilize()
	setLate(true)
	f.Stabilize()
	if loads["late"] != 1 || atoms["late"].Get() != live {
		t.Fatalf("late subscriber replaced the live filter with a stored default: %v %+v", loads, atoms["late"].Get())
	}
	setSecond(false)
	f.Stabilize()
	setSecond(true)
	f.Stabilize()
	if loads["second"] != 2 || loads["first"] != 1 || atoms["second"].Get() != live {
		t.Fatalf("remount mishandled seed or shared live state: %v %+v", loads, atoms["second"].Get())
	}
	if persisted := CurrentTxFilter(); persisted != changed {
		t.Fatalf("non-subscribing read no longer reads durable state: %+v", persisted)
	}
}

func TestTxFilterInitialLoadPreservesMigrationAndInvalidDefaults(t *testing.T) {
	app := txFilterInitialApp(t)
	now := time.Now()
	legacy := defaultTxFilterAt(now)
	legacy.PeriodPreset, legacy.Text, legacy.Account, legacy.Sort, legacy.Dir = "", "Legacy", "bank", "amount", txnfilter.Asc
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.SetKV(txFilterStoreID, string(raw)); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	loads := 0
	var atom state.Atom[TxFilter]
	var setTick func(int)
	f.Render(ui.CreateElement(func() ui.Node {
		tick := ui.UseState(0)
		setTick = tick.Set
		atom = useTxFilterWithLoader(func() TxFilter { loads++; return loadTxFilter() })
		return P(atom.Get().Text)
	}))
	want := legacy.ResolvePeriod(now).Normalize()
	if loads != 1 || atom.Get() != want {
		t.Fatalf("legacy hook seed=%+v want=%+v loads=%d", atom.Get(), want, loads)
	}
	var canonical TxFilter
	if err := json.Unmarshal([]byte(kvGet(txFilterStoreID)), &canonical); err != nil {
		t.Fatal(err)
	}
	if canonical.From != "" || canonical.To != "" || canonical.PeriodPreset != txnfilter.PeriodThisMonth || canonical.Text != legacy.Text || canonical.Account != legacy.Account || canonical.Sort != legacy.Sort {
		t.Fatalf("migration changed rolling filter contract: %+v", canonical)
	}
	revision := app.Rev()
	for tick := 1; tick <= 3; tick++ {
		setTick(tick)
		f.Stabilize()
	}
	if loads != 1 || app.Rev() != revision {
		t.Fatalf("warm renders repeated migration: loads=%d revision=%d→%d", loads, revision, app.Rev())
	}
	f.Cleanup()
	for _, raw := range []string{"", "{invalid"} {
		if err := app.SetKV(txFilterStoreID, raw); err != nil {
			t.Fatal(err)
		}
		f := render.New(t)
		f.Render(ui.CreateElement(func() ui.Node { atom = UseTxFilter(); return P(atom.Get().Text) }))
		if atom.Get() != defaultTxFilterAt(time.Now()) {
			f.Cleanup()
			t.Fatalf("absent/invalid default changed: %+v", atom.Get())
		}
		f.Cleanup()
	}
}

func TestTxFilterInitialLoadUsesFreshRuntimeWorkspaceAndRollingDates(t *testing.T) {
	first := txFilterInitialApp(t)
	second, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Store().Close()
	for _, tc := range []struct {
		app  *appstate.App
		text string
	}{{first, "First household"}, {second, "Second household"}} {
		appstate.Default = tc.app
		// Stale absolute dates with a rolling preset must resolve at mount time.
		raw, err := json.Marshal(TxFilter{Text: tc.text, PeriodPreset: txnfilter.PeriodThisMonth, From: "1999-01-01", To: "1999-01-31", Sort: "payee", Dir: txnfilter.Asc})
		if err != nil {
			t.Fatal(err)
		}
		if err := tc.app.SetKV(txFilterStoreID, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		app  *appstate.App
		text string
	}{{first, "First household"}, {second, "Second household"}, {first, "First household"}} {
		appstate.Default = tc.app
		f := render.New(t)
		loads := 0
		var got TxFilter
		f.Render(ui.CreateElement(func() ui.Node {
			atom := useTxFilterWithLoader(func() TxFilter { loads++; return loadTxFilter() })
			got = atom.Get()
			return P(got.Text)
		}))
		currentMonth := defaultTxFilterAt(time.Now())
		if loads != 1 || got.Text != tc.text || got.From != currentMonth.From || got.To != currentMonth.To || got.Sort != "payee" || got.Dir != txnfilter.Asc {
			f.Cleanup()
			t.Fatalf("workspace seed/date resolution leaked: loads=%d got=%+v", loads, got)
		}
		f.Cleanup()
	}
}
