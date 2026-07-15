// SPDX-License-Identifier: MIT

package appstate_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smoothing"
)

func smoothingRec(id string, smooth bool) domain.Recurring {
	return domain.Recurring{
		ID: id, Label: "Insurance",
		Amount:            money.New(-60000, "USD"),
		Cadence:           domain.CadenceYearly,
		NextDue:           time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
		CategoryID:        "cat1",
		SmoothIntoBudgets: smooth,
	}
}

func TestSmoothingGoalLifecycle(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	// Saving a smoothed recurring creates the managed sinking-fund goal.
	if err := app.PutRecurring(smoothingRec("r1", true)); err != nil {
		t.Fatalf("put recurring: %v", err)
	}
	g, ok := smoothing.SmoothingGoalFor(app.Goals(), "r1")
	if !ok {
		t.Fatal("expected a managed sinking-fund goal after enabling smoothing")
	}
	if g.Name != "Set aside for Insurance" {
		t.Errorf("goal name = %q, want %q", g.Name, "Set aside for Insurance")
	}
	if !g.IsSinkingFund {
		t.Error("managed goal should be a sinking fund")
	}
	if g.TargetAmount.Amount != 60000 {
		t.Errorf("target = %d, want 60000", g.TargetAmount.Amount)
	}

	// Clearing the flag dissolves the goal.
	if err := app.PutRecurring(smoothingRec("r1", false)); err != nil {
		t.Fatalf("put recurring (flag off): %v", err)
	}
	if _, ok := smoothing.SmoothingGoalFor(app.Goals(), "r1"); ok {
		t.Error("goal should be dissolved when smoothing is turned off")
	}

	// Re-enable, then delete the recurring — the goal dissolves.
	if err := app.PutRecurring(smoothingRec("r1", true)); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	if _, ok := smoothing.SmoothingGoalFor(app.Goals(), "r1"); !ok {
		t.Fatal("goal should exist after re-enabling")
	}
	if err := app.DeleteRecurring("r1"); err != nil {
		t.Fatalf("delete recurring: %v", err)
	}
	if _, ok := smoothing.SmoothingGoalFor(app.Goals(), "r1"); ok {
		t.Error("goal should be dissolved when the recurring is deleted")
	}
}
