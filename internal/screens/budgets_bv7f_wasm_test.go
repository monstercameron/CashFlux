// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestBV7FInlineLimitSavePaintsClosedEditorBeforeWriting(t *testing.T) {
	stubBudgetRowBrowser(t)
	app := guardTestApp(t)
	budget := quickFillFixtureBudget()
	budget.Name = "BV7F groceries"
	budget.Limit = money.New(45000, "USD")
	if err := app.PutBudget(budget); err != nil {
		t.Fatal(err)
	}
	props := budgetRowProps{Status: budgeting.Status{Budget: budget, Limit: budget.Limit, Remaining: budget.Limit}, Compact: true}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(BudgetRow, props))
	byTestID(f, "button", "budget-limit-btn-"+budget.ID).Click()
	f.Stabilize()
	byTestID(f, "input", "budget-limit-input-"+budget.ID).Dispatch("oninput", render.Event{Value: "470"})

	start := time.Now()
	byTestID(f, "input", "budget-limit-input-"+budget.ID).Dispatch("onkeydown", render.Event{Key: "Enter"})
	f.Stabilize()
	paint := time.Since(start)
	t.Logf("BV7F inline limit save first paint: %.3f ms", float64(paint.Microseconds())/1000)
	if paint > 150*time.Millisecond {
		t.Fatalf("inline save did not paint within 150 ms: %s", paint)
	}
	if byTestID(f, "form", "budget-limit-editor-"+budget.ID) != nil {
		t.Fatal("inline editor remained open after save was accepted")
	}
	if got := app.Budgets()[0].Limit.Amount; got != 47000 {
		t.Fatalf("optimistic local limit = %d, want 47000", got)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if got := app.Budgets()[0].Limit.Amount; got == 47000 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("deferred write did not finish; limit = %d", app.Budgets()[0].Limit.Amount)
}
