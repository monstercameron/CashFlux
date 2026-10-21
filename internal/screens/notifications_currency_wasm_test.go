// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestPrimaryNotificationGoalWinsRefreshWhenAllocationRatesChange(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", map[string]float64{"EUR": .5}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	goal := domain.Goal{ID: "fx-goal", Name: "Foreign allocation reserve", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		CurrentAmount: money.New(0, "USD"), TargetAmount: money.New(10000, "USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "euro", Amount: money.New(10000, "EUR")}}}
	if err := app.PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(notifWinsWidget, notifProps{App: app}))
	if strings.Contains(f.Text(), goal.Name) {
		t.Fatal("foreign earmark below its converted target was celebrated")
	}
	settings.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(f.Text(), goal.Name) || !strings.Contains(uistate.KVGet(winsSeenKey), "goal:"+goal.Name) {
		t.Fatal("rate revision did not celebrate the known-complete goal with its original seen key")
	}
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if strings.Contains(f.Text(), goal.Name) {
		t.Fatal("unknown allocation FX left a falsely completed goal milestone visible")
	}
}
