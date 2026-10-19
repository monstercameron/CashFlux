// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSOLB2EditorSavesHistoryInViewedPeriod(t *testing.T) {
	app := guardTestApp(t)
	b := domain.Budget{ID: "solb2", Name: "Transport", CategoryID: "c-transport", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), Rollover: true, RolloverStart: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	// Seed the recorded funding period explicitly; ordinary creation starts at
	// its effective date rather than accepting a caller-supplied older start.
	if err := app.PutBudgetAt(b, b.RolloverStart); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	atom := state.NewGlobalAtom("dashboard:period", period.Window{})
	previous := atom.Get()
	defer atom.Set(previous)
	viewed := time.Date(2030, 11, 1, 0, 0, 0, 0, time.UTC)
	atom.Set(period.NewWindow(period.Month, viewed, time.Monday))
	saved := false
	f.Render(ui.CreateElement(BudgetEditForm, BudgetEditFormProps{BudgetID: b.ID, Mode: uistate.BudgetEditModeEdit, OnDone: func() { saved = true }}))
	limit := byTestID(f, "input", "budget-edit-limit")
	if limit == nil {
		t.Fatal("missing limit")
	}
	limit.Input("200.00")
	f.Stabilize()
	forms := f.AllByTag("form")
	if len(forms) == 0 {
		t.Fatal("missing save form")
	}
	forms[0].Submit()
	f.Stabilize()
	got := app.Budgets()[0]
	if !saved || got.Limit.Amount != 20000 || got.LimitHistory["2026-08-01"] != 10000 || got.LimitHistory["2030-11-01"] != 20000 || got.RolloverStart != b.RolloverStart {
		t.Fatalf("saved=%v budget=%+v", saved, got)
	}
}
