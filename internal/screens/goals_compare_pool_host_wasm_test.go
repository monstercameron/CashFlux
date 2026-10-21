// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestGoalCompareOrderPoolHostKeepsOverflowUnavailable(t *testing.T) {
	f := render.New(t)
	defer func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	f.Render(goalCompareOrderPanel(goalCompareOrderProps{
		GoalA: domain.Goal{Name: "A"}, GoalB: domain.Goal{Name: "B"},
		MonthlyA: money.New(math.MaxInt64, "USD"), MonthlyB: money.New(1, "USD"),
		RemainingA: money.New(100, "USD"), RemainingB: money.New(200, "USD"),
		HasMonthlyA: true, HasMonthlyB: true,
	}))
	f.Stabilize()
	if byTestID(f, "p", "goal-compare-order-unavailable") == nil {
		t.Fatalf("overflowing pool was not marked unavailable: %s", f.Text())
	}
	if byTestID(f, "table", "goal-compare-order") != nil || byTestID(f, "p", "goal-compare-order-moot") != nil {
		t.Fatalf("overflowing pool rendered a plausible comparison: %s", f.Text())
	}
}

func TestGoalCompareOrderPoolHostPreservesKnownFittingPlans(t *testing.T) {
	f := render.New(t)
	defer func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	f.Render(goalCompareOrderPanel(goalCompareOrderProps{
		GoalA: domain.Goal{Name: "A"}, GoalB: domain.Goal{Name: "B"},
		MonthlyA: money.New(10_000, "USD"), MonthlyB: money.New(10_000, "USD"),
		RemainingA: money.New(30_000, "USD"), RemainingB: money.New(20_000, "USD"),
		HasMonthlyA: true, HasMonthlyB: true,
	}))
	f.Stabilize()
	// The pool is the sum of both monthly caps, so both existing plans fit;
	// the native comparison correctly says that changing priority has no effect.
	if byTestID(f, "p", "goal-compare-order-moot") == nil || byTestID(f, "p", "goal-compare-order-unavailable") != nil {
		t.Fatalf("known fitting plans lost their no-order-change result: %s", f.Text())
	}
}
