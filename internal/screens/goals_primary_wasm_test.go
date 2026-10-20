// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestPrimaryGoalSummaryUsesConvertedTotalsAndDisclosesExclusions(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	settings := store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, g := range []domain.Goal{
		{ID: "usd", Name: "USD", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.New(10000, "USD"), TargetAmount: money.New(10000, "USD")},
		{ID: "eur", Name: "EUR", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.New(0, "EUR"), TargetAmount: money.New(10000, "EUR")},
		{ID: "gbp", Name: "GBP", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, CurrentAmount: money.New(100000, "GBP"), TargetAmount: money.New(100000, "GBP")},
	} {
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(goalSummaryWidget, goalSummaryProps{App: app}))
	f.Stabilize()
	if text := f.Text(); !strings.Contains(text, "33%") || !strings.Contains(text, "300.00") {
		t.Fatalf("summary does not reconcile with converted totals: %s", text)
	}
	note := byTestID(f, "p", "goal-summary-excluded-currencies")
	if note == nil || !strings.Contains(note.Text(), "GBP") {
		t.Fatalf("missing-FX omission is not disclosed: %v", note)
	}
	revision := app.Rev()
	settings.FXRates["GBP"] = 1
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if app.Rev() == revision {
		t.Fatal("settings write did not advance the store revision")
	}
	// Verify the model's store-revision invalidation independently of rendering.
	updated := computeGoalView(app, "")
	if len(updated.ExcludedCurrencies) != 0 || updated.TargetTotal.Amount != 130000 || updated.OverallPct != 84 {
		t.Fatalf("settings revision retained a stale goal model: %+v", updated)
	}
	// Real settings handlers notify subscribers after saving. Re-rendering the
	// same component with the same App pointer can be skipped by reconciliation.
	uistate.BumpDataRevision()
	f.Stabilize()
	if byTestID(f, "p", "goal-summary-excluded-currencies") != nil {
		t.Fatal("excluded currency note survived the settings revision")
	}
	if text := f.Text(); !strings.Contains(text, "84%") || !strings.Contains(text, "1,300.00") {
		t.Fatalf("summary did not update after the settings notification: %s", text)
	}
}
