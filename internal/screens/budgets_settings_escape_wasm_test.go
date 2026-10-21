// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type budgetEscapeCSSSink struct{}

func (budgetEscapeCSSSink) Emit(string, string) {}

func TestBudgetSettingsEscapeAfterSortClosesActualToolbar(t *testing.T) {
	app := bud2TestApp(t)
	defer app.Store().Close()
	for _, id := range []string{"first", "second"} {
		if err := app.PutBudget(domain.Budget{ID: id, Name: id, CategoryID: "c-food", OwnerID: domain.GroupOwnerID,
			Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(1000, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	previousCSS := css.SetSink(budgetEscapeCSSSink{})
	global := js.Global()
	previous := make(map[string]js.Value)
	for _, name := range []string{"document", "requestAnimationFrame", "cancelAnimationFrame", "addEventListener", "removeEventListener"} {
		previous[name] = global.Get(name)
	}
	var owned []js.Func
	callback := func(fn func(js.Value, []js.Value) any) js.Func {
		value := js.FuncOf(fn)
		owned = append(owned, value)
		return value
	}
	listeners := make(map[string][]js.Value)
	document := global.Get("Object").New()
	document.Set("getElementById", callback(func(js.Value, []js.Value) any { return js.Null() }))
	document.Set("addEventListener", callback(func(_ js.Value, args []js.Value) any {
		listeners[args[0].String()] = append(listeners[args[0].String()], args[1])
		return nil
	}))
	document.Set("removeEventListener", callback(func(_ js.Value, args []js.Value) any {
		name := args[0].String()
		kept := listeners[name][:0]
		for _, listener := range listeners[name] {
			if !listener.Equal(args[1]) {
				kept = append(kept, listener)
			}
		}
		listeners[name] = kept
		return nil
	}))
	noop := callback(func(js.Value, []js.Value) any { return nil })
	global.Set("document", document)
	global.Set("requestAnimationFrame", callback(func(js.Value, []js.Value) any { return 1 }))
	global.Set("cancelAnimationFrame", noop)
	global.Set("addEventListener", noop)
	global.Set("removeEventListener", noop)
	defer func() {
		f.Cleanup()
		css.SetSink(previousCSS)
		for name, value := range previous {
			global.Set(name, value)
		}
		for _, fn := range owned {
			fn.Release()
		}
	}()

	// Use the production list and toolbar. Its real sort atom notification updates
	// the ancestor list and child Settings props while the menu stays open.
	f.Render(ui.CreateElement(budgetListWidget, budgetListProps{App: app}))
	settings := byTestID(f, "button", "budgets-settings")
	if settings == nil {
		t.Fatal("actual budget toolbar did not render")
	}
	byTestID(f, "select", "budgets-sort").Dispatch("onchange", render.Event{Value: uistate.BudgetSortHealth})
	f.Stabilize()
	byTestID(f, "button", "budgets-settings").Click()
	f.Stabilize()
	if byTestID(f, "button", "budgets-settings").Attr("aria-expanded") != "true" {
		t.Fatal("settings did not open")
	}
	byTestID(f, "select", "budgets-sort").Dispatch("onchange", render.Event{Value: uistate.BudgetSortOverage})
	f.Stabilize()
	selectedSort := ""
	for _, option := range byTestID(f, "select", "budgets-sort").Children() {
		if selected, _ := option.Property("selected").(bool); selected {
			selectedSort, _ = option.Property("value").(string)
			if selectedSort == "" {
				selectedSort = option.Attr("value")
			}
		}
	}
	if selectedSort != uistate.BudgetSortOverage {
		t.Fatal("production sort handler did not update the toolbar")
	}
	if len(listeners["keydown"]) == 0 {
		t.Fatal("open Settings lost its document Escape listener after sorting")
	}
	event := global.Get("Object").New()
	event.Set("key", "Escape")
	for _, listener := range append([]js.Value(nil), listeners["keydown"]...) {
		listener.Invoke(event)
	}
	f.Stabilize()
	if byTestID(f, "button", "budgets-settings").Attr("aria-expanded") != "false" || !strings.Contains(byTestID(f, "div", "budgets-settings-menu").Attr("class"), "hidden-menu") {
		t.Fatal("Escape after sorting left the production Settings menu open")
	}
}
