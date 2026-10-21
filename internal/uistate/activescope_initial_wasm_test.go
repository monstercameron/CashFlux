// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/scope"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func activeScopeInitialApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	previousApp, previousAtom, previousCaptured := appstate.Default, capturedActiveScope, activeScopeCaptured
	appstate.Default, activeScopeCaptured = app, false
	t.Cleanup(func() {
		appstate.Default, capturedActiveScope, activeScopeCaptured = previousApp, previousAtom, previousCaptured
		_ = app.Store().Close()
	})
	return app
}

type activeScopeReaderProps struct {
	Name string
	Tick int
}

func TestActiveScopeInitialLoadRunsOncePerMountedHook(t *testing.T) {
	app := activeScopeInitialApp(t)
	stored := scope.ReportScope{Owners: []string{"alex"}, Institutions: []string{"First Bank"}, Types: []domain.AccountType{domain.TypeChecking}, AccountIDs: []string{"cash"}}
	persistActiveScope(stored)
	loads := map[string]int{}
	atoms := map[string]state.Atom[scope.ReportScope]{}
	reader := func(p activeScopeReaderProps) ui.Node {
		a := useActiveScopeWithLoader(func() scope.ReportScope { loads[p.Name]++; return loadActiveScope() })
		atoms[p.Name] = a
		return P(Attr("id", p.Name), strings.Join(a.Get().Owners, ","))
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
			secondNode = ui.CreateElement(reader, activeScopeReaderProps{Name: "second", Tick: tick.Get()})
		}
		if late.Get() {
			lateNode = ui.CreateElement(reader, activeScopeReaderProps{Name: "late", Tick: tick.Get()})
		}
		return Div(ui.CreateElement(reader, activeScopeReaderProps{Name: "first", Tick: tick.Get()}), Div(secondNode), Div(lateNode))
	}))
	if loads["first"] != 1 || loads["second"] != 1 || !reflect.DeepEqual(atoms["first"].Get(), stored) {
		t.Fatalf("initial scope loads=%v scope=%+v", loads, atoms["first"].Get())
	}
	for tick := 1; tick <= 3; tick++ {
		setTick(tick)
		f.Stabilize()
	}
	changed := scope.ReportScope{Owners: []string{"blair"}, Institutions: stored.Institutions, Types: stored.Types, AccountIDs: stored.AccountIDs}
	SetActiveScope(changed)
	f.Stabilize()
	if !reflect.DeepEqual(atoms["second"].Get(), changed) || f.ByID("first").Text() != "blair" || ActiveMemberFromScope() != "blair" {
		t.Fatal("captured setter did not update subscribers and persistence")
	}
	if err := app.SetKV("scope-test:unrelated", "revision"); err != nil {
		t.Fatal(err)
	}
	setTick(4)
	f.Stabilize()
	if loads["first"] != 1 || loads["second"] != 1 {
		t.Fatalf("warm/store-revision renders reread ignored scope defaults: %v", loads)
	}
	live := scope.ReportScope{Owners: []string{"live-only"}, AccountIDs: []string{"unpersisted"}}
	atoms["first"].Set(live)
	f.Stabilize()
	setLate(true)
	f.Stabilize()
	if loads["late"] != 1 || !reflect.DeepEqual(atoms["late"].Get(), live) {
		t.Fatalf("late seed replaced live scope: %v %+v", loads, atoms["late"].Get())
	}
	setSecond(false)
	f.Stabilize()
	setSecond(true)
	f.Stabilize()
	if loads["second"] != 2 || loads["first"] != 1 || !reflect.DeepEqual(atoms["second"].Get(), live) {
		t.Fatalf("remount replaced the fixed atom or reused a seed: %v %+v", loads, atoms["second"].Get())
	}
	// The setter must still target the current fixed atom after late mounts.
	SetActiveScope(changed)
	f.Stabilize()
	ClearMemberLens()
	f.Stabilize()
	withoutOwners := changed
	withoutOwners.Owners = nil
	if !reflect.DeepEqual(atoms["first"].Get(), withoutOwners) || !reflect.DeepEqual(loadActiveScope(), withoutOwners) || len(atoms["late"].Get().Owners) != 0 {
		t.Fatalf("captured member-lens reset lost other scope dimensions: %+v", atoms["first"].Get())
	}
}

func TestActiveScopeInitialLoadPreservesExactLegacyMigrationAndInvalidDefaults(t *testing.T) {
	canonical := scope.ReportScope{Owners: []string{"current", "second"}, Institutions: []string{"A B", "C"}, Types: []domain.AccountType{domain.TypeSavings}, AccountIDs: []string{"named"}}
	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, raw, member string
		want              scope.ReportScope
		migrate           bool
	}{
		{"legacy only", "", "alex", scope.ReportScope{Owners: []string{"alex"}}, true},
		{"default Everyone", "", "", scope.ReportScope{}, false},
		{"current scope wins", string(canonicalJSON), "legacy", canonical, false},
		{"invalid scope blocks migration", "{invalid", "legacy", scope.ReportScope{}, false},
		{"null scope blocks migration", "null", "legacy", scope.ReportScope{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := activeScopeInitialApp(t)
			if err := app.SetKV(activeScopeStore, tc.raw); err != nil {
				t.Fatal(err)
			}
			if err := app.SetKV(activeMemberStore, tc.member); err != nil {
				t.Fatal(err)
			}
			// Before capture, the public setter deliberately does nothing.
			SetActiveScope(scope.ReportScope{Owners: []string{"must-not-write"}})
			beforeRevision := app.Rev()
			f := render.New(t, render.WithQueuedScheduler())
			defer f.Cleanup()
			loads := 0
			var atom state.Atom[scope.ReportScope]
			var setTick func(int)
			f.Render(ui.CreateElement(func() ui.Node {
				tick := ui.UseState(0)
				setTick = tick.Set
				atom = useActiveScopeWithLoader(func() scope.ReportScope { loads++; return loadActiveScope() })
				return P(strings.Join(atom.Get().Owners, ","))
			}))
			if loads != 1 || !reflect.DeepEqual(atom.Get(), tc.want) {
				t.Fatalf("scope migration/default=%+v want=%+v loads=%d", atom.Get(), tc.want, loads)
			}
			raw, _ := app.GetKV(activeScopeStore)
			member, memberPresent := app.GetKV(activeMemberStore)
			if tc.migrate {
				var persisted scope.ReportScope
				if json.Unmarshal([]byte(raw), &persisted) != nil || !reflect.DeepEqual(persisted, tc.want) || memberPresent || app.Rev()-beforeRevision != 2 {
					t.Fatalf("legacy migration must persist scope and remove only legacy key: raw=%q member=%q present=%v revision=%d→%d", raw, member, memberPresent, beforeRevision, app.Rev())
				}
			} else if raw != tc.raw || !memberPresent || member != tc.member || app.Rev() != beforeRevision {
				t.Fatalf("default/current/invalid seed mutated persistence: raw=%q member=%q revision=%d→%d", raw, member, beforeRevision, app.Rev())
			}
			revision := app.Rev()
			for tick := 1; tick <= 3; tick++ {
				setTick(tick)
				f.Stabilize()
			}
			if loads != 1 || app.Rev() != revision {
				t.Fatalf("warm renders repeated migration/default loads: loads=%d revision=%d→%d", loads, revision, app.Rev())
			}
		})
	}
}

func TestActiveScopeInitialLoadUsesFreshRuntimeWorkspaceAndCapturedSetter(t *testing.T) {
	first := activeScopeInitialApp(t)
	second, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Store().Close()
	wants := map[*appstate.App]scope.ReportScope{
		first:  {Owners: []string{"first"}, Institutions: []string{"Alpha"}},
		second: {Owners: []string{"second"}, Institutions: []string{"Beta"}},
	}
	for app, value := range wants {
		appstate.Default = app
		persistActiveScope(value)
	}
	for _, app := range []*appstate.App{first, second, first} {
		appstate.Default = app
		f := render.New(t, render.WithQueuedScheduler())
		loads := 0
		var atom state.Atom[scope.ReportScope]
		var memberLens []string
		f.Render(ui.CreateElement(func() ui.Node {
			atom = useActiveScopeWithLoader(func() scope.ReportScope { loads++; return loadActiveScope() })
			memberLens = UseMemberLens()
			return P(strings.Join(memberLens, ","))
		}))
		if loads != 1 || !reflect.DeepEqual(atom.Get(), wants[app]) || !reflect.DeepEqual(memberLens, wants[app].Owners) {
			f.Cleanup()
			t.Fatalf("workspace scope leaked: loads=%d got=%+v lens=%v want=%+v", loads, atom.Get(), memberLens, wants[app])
		}
		changed := scope.ReportScope{Institutions: wants[app].Institutions, Owners: []string{wants[app].Owners[0] + "-live"}}
		SetActiveScope(changed)
		f.Stabilize()
		if loads != 1 || !reflect.DeepEqual(atom.Get(), changed) || !reflect.DeepEqual(memberLens, changed.Owners) || !reflect.DeepEqual(loadActiveScope(), changed) {
			f.Cleanup()
			t.Fatalf("fresh-runtime captured setter points at old registry: loads=%d got=%+v lens=%v", loads, atom.Get(), memberLens)
		}
		wants[app] = changed
		f.Cleanup()
	}
}
