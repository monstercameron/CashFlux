// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func assertGoalsTabs(t *testing.T, f *render.Fixture, selected string) {
	t.Helper()
	group := f.ByRole("tablist", uistate.T("goals.toolbarLabel"))
	if group == nil || group.Attr("class") != "goals-tabs" || len(f.AllByRole("tab")) != 2 {
		t.Fatal("fixed Goals tab group lost its classes, accessible name or two choices")
	}
	for _, option := range []struct{ key, label string }{
		{uistate.GoalsViewGoals, uistate.T("goals.viewGoals")},
		{uistate.GoalsViewEarmarks, uistate.T("goals.viewEarmarks")},
	} {
		tab := byTestID(f, "button", "goals-tab-"+option.key)
		if tab == nil || tab.Text() != option.label || tab.Attr("role") != "tab" || tab.Attr("aria-pressed") != "" {
			t.Fatalf("%s tab changed its label/uncounted content or tab semantics", option.key)
		}
		for _, child := range tab.Children() {
			if child.Tag() == "span" {
				t.Fatalf("fixed %s tab gained a count badge", option.key)
			}
		}
		class, tabIndex, ariaSelected := "goals-tab", "-1", "false"
		if option.key == selected {
			class, tabIndex, ariaSelected = "goals-tab is-active", "0", "true"
		}
		if tab.Attr("class") != class || tab.Attr("tabindex") != tabIndex || tab.Attr("aria-selected") != ariaSelected {
			t.Fatalf("%s tab has class=%q tabindex=%q selected=%q", option.key, tab.Attr("class"), tab.Attr("tabindex"), tab.Attr("aria-selected"))
		}
	}
	if (byTestID(f, "select", "goals-sort") != nil) != (selected == uistate.GoalsViewGoals) {
		t.Fatal("tab selection no longer controls the Goals-only sort picker")
	}
}

// The testkit DOM is separate from JavaScript document lookup. Record requests
// for the actual rendered tab IDs while retaining other document behavior.
func observeGoalsTabFocus(t *testing.T, f *render.Fixture) *string {
	t.Helper()
	previousDocument := js.Global().Get("document")
	document := previousDocument
	if !document.Truthy() {
		document = js.Global().Get("Object").New()
	}
	previousLookup := document.Get("getElementById")
	ids := map[string]bool{}
	for _, key := range []string{uistate.GoalsViewGoals, uistate.GoalsViewEarmarks} {
		ids[byTestID(f, "button", "goals-tab-"+key).Attr("id")] = true
	}
	requested, focused := "", new(string)
	style := js.Global().Get("Object").New()
	style.Set("textContent", "")
	focus := js.FuncOf(func(js.Value, []js.Value) any { *focused = requested; return nil })
	lookup := js.FuncOf(func(_ js.Value, args []js.Value) any {
		requested = args[0].String()
		if requested == "gwc-css" {
			return style // CSS emitted by newly mounted Earmarks content
		}
		if ids[requested] {
			element := js.Global().Get("Object").New()
			element.Set("focus", focus)
			return element
		}
		if previousLookup.Type() == js.TypeFunction {
			return previousLookup.Call("call", document, requested)
		}
		return js.Null()
	})
	document.Set("getElementById", lookup)
	js.Global().Set("document", document)
	t.Cleanup(func() {
		if previousLookup.Type() == js.TypeUndefined {
			document.Delete("getElementById")
		} else {
			document.Set("getElementById", previousLookup)
		}
		js.Global().Set("document", previousDocument)
		focus.Release()
		lookup.Release()
	})
	return focused
}

func TestGoalsSegmentedTabsKeyboardFocusAndActualContent(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	goal := domain.Goal{ID: "tab-trip", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD")}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Goals))
	f.Stabilize()
	primaryClick(t, f, "goals-tab-goals")
	assertGoalsTabs(t, f, uistate.GoalsViewGoals)
	focused := observeGoalsTabFocus(t, f)
	ids := map[string]string{}
	for _, key := range []string{uistate.GoalsViewGoals, uistate.GoalsViewEarmarks} {
		ids[key] = byTestID(f, "button", "goals-tab-"+key).Attr("id")
	}
	key := func(from, key, want string) {
		t.Helper()
		byTestID(f, "button", "goals-tab-"+from).Dispatch("onkeydown", render.Event{Key: key})
		f.Stabilize()
		assertGoalsTabs(t, f, want)
		if *focused != ids[want] || byTestID(f, "button", "goals-tab-"+want).Attr("id") != ids[want] {
			t.Fatalf("%s on %s did not focus its stable selected tab: %q", key, from, *focused)
		}
		if row := byTestID(f, "div", "goal-row-"+goal.ID); (row != nil) != (want == uistate.GoalsViewGoals) {
			t.Fatal("keyboard selection did not switch the actual Goals/Earmarks content")
		}
	}
	key("goals", "ArrowRight", "earmarks")
	key("earmarks", "ArrowRight", "goals")
	key("goals", "ArrowLeft", "earmarks")
	key("earmarks", "Home", "goals")
	key("goals", "End", "earmarks")
	byTestID(f, "button", "goals-tab-earmarks").Dispatch("onkeydown", render.Event{Key: "ArrowDown"})
	f.Stabilize()
	assertGoalsTabs(t, f, uistate.GoalsViewEarmarks)
	primaryClick(t, f, "goals-tab-goals")
	assertGoalsTabs(t, f, uistate.GoalsViewGoals)
	key("goals", "End", "earmarks")
}

func TestGoalsSegmentedTabsRetainUncountedLabelsThroughDataAndModeTransitions(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(Goals))
	f.Stabilize()
	primaryClick(t, f, "goals-tab-goals")
	assertGoalsTabs(t, f, uistate.GoalsViewGoals)
	goalsID := byTestID(f, "button", "goals-tab-goals").Attr("id")
	earmarksID := byTestID(f, "button", "goals-tab-earmarks").Attr("id")
	primaryClick(t, f, "goals-tab-earmarks")
	assertGoalsTabs(t, f, uistate.GoalsViewEarmarks)
	if err := app.PutAccount(domain.Account{ID: "tab-bank", Name: "Bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(20000, "USD")}); err != nil {
		t.Fatal(err)
	}
	goal := domain.Goal{ID: "tab-trip", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		CurrentAmount: money.Zero("USD"), TargetAmount: money.New(10000, "USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "tab-bank", Amount: money.New(2000, "USD")}}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	assertGoalsTabs(t, f, uistate.GoalsViewEarmarks)
	if byTestID(f, "div", "ea-goal-"+goal.ID) == nil {
		t.Fatal("selected Earmarks tab did not refresh from empty to populated content")
	}
	for cycle := 0; cycle < 2; cycle++ {
		primaryClick(t, f, "goals-tab-goals")
		assertGoalsTabs(t, f, uistate.GoalsViewGoals)
		if byTestID(f, "div", "goal-row-"+goal.ID) == nil {
			t.Fatal("Goals tab lost its actual populated content")
		}
		primaryClick(t, f, "goals-tab-earmarks")
		assertGoalsTabs(t, f, uistate.GoalsViewEarmarks)
	}
	if byTestID(f, "button", "goals-tab-goals").Attr("id") != goalsID || byTestID(f, "button", "goals-tab-earmarks").Attr("id") != earmarksID {
		t.Fatal("data and conditional-content changes replaced fixed tab identities")
	}
}
