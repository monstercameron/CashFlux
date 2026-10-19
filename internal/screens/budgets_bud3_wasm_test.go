// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func stubBudgetRowBrowser(t *testing.T) {
	t.Helper()
	global := js.Global()
	previous := map[string]js.Value{}
	for _, name := range []string{"document", "requestAnimationFrame", "cancelAnimationFrame", "addEventListener", "removeEventListener"} {
		previous[name] = global.Get(name)
	}
	var funcs []js.Func
	noop := func(js.Value, []js.Value) any { return nil }
	makeFunc := func(fn func(js.Value, []js.Value) any) js.Func {
		cb := js.FuncOf(fn)
		funcs = append(funcs, cb)
		return cb
	}
	focusTarget := js.Global().Get("Object").New()
	focusTarget.Set("focus", makeFunc(noop))
	document := js.Global().Get("Object").New()
	document.Set("addEventListener", makeFunc(noop))
	document.Set("removeEventListener", makeFunc(noop))
	document.Set("getElementById", makeFunc(func(js.Value, []js.Value) any { return js.Null() }))
	styleElement := js.Global().Get("Object").New()
	styleElement.Set("setAttribute", makeFunc(noop))
	document.Set("createElement", makeFunc(func(js.Value, []js.Value) any { return styleElement }))
	head := js.Global().Get("Object").New()
	head.Set("appendChild", makeFunc(noop))
	document.Set("head", head)
	document.Set("querySelector", makeFunc(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && strings.Contains(args[0].String(), "budget-limit-btn-") {
			return focusTarget
		}
		return js.Null()
	}))
	global.Set("document", document)
	global.Set("requestAnimationFrame", makeFunc(func(js.Value, []js.Value) any { return 1 }))
	global.Set("cancelAnimationFrame", makeFunc(noop))
	global.Set("addEventListener", makeFunc(noop))
	global.Set("removeEventListener", makeFunc(noop))
	t.Cleanup(func() {
		for name, value := range previous {
			global.Set(name, value)
		}
		for _, fn := range funcs {
			fn.Release()
		}
	})
}

func TestBud3CompactLimitEditAndRolloverMenu(t *testing.T) {
	stubBudgetRowBrowser(t)
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	b.Limit = money.New(45000, "USD")
	if err := app.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	props := budgetRowProps{Status: budgeting.Status{
		Budget: b, Limit: money.New(20520, "USD"), Spent: money.New(10000, "USD"),
		Remaining: money.New(10520, "USD"), Percent: 48, State: budgeting.StateOK,
	}, Compact: true, OnDelete: func(string) {}}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, props))
	limit := byTestID(f, "button", "budget-limit-btn-"+b.ID)
	if limit == nil {
		t.Fatal("compact row does not expose a clickable limit")
	}
	if byTestID(f, "button", "budget-release-btn-"+b.ID) != nil {
		t.Fatal("non-rollover budget exposes Release unused funds")
	}
	byTestID(f, "button", "budget-kebab-"+b.ID).Click()
	f.Stabilize()
	if got := byTestID(f, "button", "budget-topup-btn-"+b.ID); got == nil || got.Text() != uistate.T("budgets.actionRaiseLimit") {
		t.Fatalf("top-up menu action = %v, want the plain Raise limit label", got)
	}
	if got := byTestID(f, "button", "edit-budget-cats-btn-"+b.ID); got == nil || got.Text() != uistate.T("budgets.actionEditTracking") {
		t.Fatalf("tracking menu action = %v, want Edit tracking", got)
	}
	if got := byTestID(f, "button", "budget-formulas-btn-"+b.ID); got == nil || got.Text() != uistate.T("budgets.actionEditFormulas") {
		t.Fatalf("formulas menu action = %v, want Edit formulas", got)
	}
	byTestID(f, "button", "budget-kebab-"+b.ID).Click()
	f.Stabilize()
	byTestID(f, "button", "budget-limit-btn-"+b.ID).Click()
	f.Stabilize()
	if editor := byTestID(f, "form", "budget-limit-editor-"+b.ID); editor == nil {
		t.Fatal("limit click did not open the inline editor")
	}
	if got := byTestID(f, "strong", "budget-limit-effective-"+b.ID); got == nil || got.Text() != "$205.20" {
		t.Fatalf("effective cap = %v, want $205.20", got)
	}
	if got := byTestID(f, "input", "budget-limit-input-"+b.ID); got == nil || got.Attr("aria-label") != uistate.T("budgets.baseLimitLabel") {
		t.Fatal("base limit input is missing its explicit label")
	}
	if text := byTestID(f, "form", "budget-limit-editor-"+b.ID).Text(); !strings.Contains(text, "$205.20") || !strings.Contains(text, "$450.00") {
		t.Fatalf("editor does not show effective and base limits together: %q", text)
	}
	byTestID(f, "input", "budget-limit-input-"+b.ID).Dispatch("onkeydown", render.Event{Key: "Escape"})
	f.Stabilize()
	if byTestID(f, "form", "budget-limit-editor-"+b.ID) != nil {
		t.Fatal("Escape did not cancel the editor")
	}
	byTestID(f, "button", "budget-limit-btn-"+b.ID).Click()
	f.Stabilize()
	input := byTestID(f, "input", "budget-limit-input-"+b.ID)
	input.Dispatch("oninput", render.Event{Value: "470"})
	input.Dispatch("onkeydown", render.Event{Key: "Enter"})
	f.Stabilize()
	if got := app.Budgets()[0].Limit.Amount; got != 47000 {
		t.Fatalf("Enter saved limit %d minor units, want 47000", got)
	}
}

func TestBud3PaceLegendAppearsOnlyForSpentBudgets(t *testing.T) {
	b := quickFillFixtureBudget()
	props := budgetRowProps{Status: budgeting.Status{Budget: b, Limit: b.Limit, Remaining: b.Limit}, PaceMarkerPct: 70}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, props))
	if byTestID(f, "span", "budget-pace-legend-"+b.ID) != nil {
		t.Fatal("pace legend appeared when nothing has been spent")
	}
	props.Status.Spent = money.New(1000, "USD")
	f.Rerender(ui.CreateElement(BudgetRow, props))
	legend := byTestID(f, "span", "budget-pace-legend-"+b.ID)
	if legend == nil || legend.Text() == "" {
		t.Fatal("spent budget is missing the pace legend")
	}
}

func TestBud3CompactOverAmountIsPlainText(t *testing.T) {
	guardTestApp(t)
	b := quickFillFixtureBudget()
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, budgetRowProps{Status: budgeting.Status{
		Budget: b, Limit: b.Limit, Spent: money.New(24250, "USD"), Remaining: money.New(-3730, "USD"),
		Percent: 118, State: budgeting.StateOver,
	}, Compact: true, OnDelete: func(string) {}}))
	for _, node := range f.AllByTag("span") {
		if strings.Contains(node.Attr("class"), "budget-crow-left") && node.Text() == "$37.30 over" {
			return
		}
	}
	t.Fatal("compact over-budget amount is missing the readable '$37.30 over' text")
}
