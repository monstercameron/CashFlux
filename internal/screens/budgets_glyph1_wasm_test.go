// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// TestBudgetGoalGlyphMountsAndUnmountsWithoutEffectPanic covers the row's
// conditional marker paths. The testkit has no browser document, so a glyph's
// optional native keyboard bridge must remain safe while the row changes shape.
func TestBudgetGoalGlyphMountsAndUnmountsWithoutEffectPanic(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	b := quickFillFixtureBudget()
	props := budgetRowProps{
		Status: budgeting.Status{
			Budget: b, Limit: b.Limit, Spent: money.Zero("USD"), Remaining: b.Limit,
		},
		Category: "Transportation",
		Compact:  true,
		OnDelete: func(string) {},
	}

	assertNoGlyphPanic := func(stage string) {
		t.Helper()
		for _, diagnostic := range f.BuildDiagnostics() {
			if diagnostic.Code == "GWC-RUNTIME-PANIC-EFFECT" && diagnostic.Message != "" {
				t.Errorf("%s logged an effect panic: %s", stage, diagnostic.Message)
			}
		}
		for _, entry := range f.BuildLogs() {
			if entry.Code == "GWC-RUNTIME-PANIC-EFFECT" || entry.Classification == "panic" {
				t.Errorf("%s logged an effect panic: %s", stage, entry.Message)
			}
		}
	}

	// Rollover exercises the nested badge call site that exposed the original
	// panic. Turning Covered on and off also exercises the conditional glyph
	// siblings that share the row head.
	b.Rollover = true
	props.Status.Budget = b
	f.Render(ui.CreateElement(BudgetRow, props))
	if byTestID(f, "button", "budget-glyph-rollover-"+b.ID) == nil {
		t.Fatal("the rollover glyph did not mount")
	}
	assertNoGlyphPanic("rollover mount")

	b.Rollover = false
	props.Status.Budget = b
	props.Covered = true
	f.Rerender(ui.CreateElement(BudgetRow, props))
	if byTestID(f, "button", "budget-glyph-cover-"+b.ID) == nil {
		t.Fatal("the one-time coverage glyph did not mount after the rollover glyph unmounted")
	}
	assertNoGlyphPanic("coverage mount and rollover unmount")

	props.Covered = false
	f.Rerender(ui.CreateElement(BudgetRow, props))
	if byTestID(f, "button", "budget-glyph-cover-"+b.ID) != nil {
		t.Fatal("the coverage glyph remained mounted after its condition became false")
	}
	assertNoGlyphPanic("coverage unmount")

	f.Rerender(ui.Fragment())
	assertNoGlyphPanic("row unmount")
}
