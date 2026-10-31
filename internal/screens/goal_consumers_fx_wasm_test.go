// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func savingsGoalCoverageFixture(row savingsAcct) ui.Node {
	return budgetSavingsGoalNode(row, ui.Handler{})
}

func dashboardGoalCoverageFixture(app *appstate.App) ui.Node {
	return goalsWidget(app, widgetcfg.Config{})
}

func TestBudgetSavingsUnknownCoverageRendersUnavailableWithoutSync(t *testing.T) {
	app := bud2TestApp(t)
	defer app.Store().Close()
	account := domain.Account{ID: "savings", Name: "Savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD", OpeningBalance: money.Zero("USD"), MonthlySavings: money.New(100, "USD")}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	goal := domain.Goal{ID: "legacy", Name: "Trip", AccountID: account.ID, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(2000, "EUR")}
	// Imported legacy amounts can carry a different currency from the target.
	if err := app.Store().PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	rows := computeSavingsAccounts(app, "", time.Now(), "USD", currency.Rates{Base: "USD"})
	if len(rows) != 1 {
		t.Fatalf("expected linked savings row, got %+v", rows)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(savingsGoalCoverageFixture, rows[0]))
	f.Stabilize()
	if byTestID(f, "span", "budgets-savings-goal-unavailable-savings") == nil || !strings.Contains(f.Text(), "Trip") {
		t.Fatalf("unknown goal coverage did not render explicitly: %s", f.Text())
	}
	if byTestID(f, "button", "budgets-savings-sync-savings") != nil || strings.Contains(f.Text(), uistate.T("budgets.savingsFunded")) {
		t.Fatalf("unknown goal coverage was presented as funded or syncable: %s", f.Text())
	}
	if !reflect.DeepEqual(app.Goals()[0], goal) {
		t.Fatal("rendering rewrote a foreign saved amount")
	}
}

func TestDashboardGoalCoverageRefreshesAfterFXSettingsChange(t *testing.T) {
	app := bud2TestApp(t)
	defer app.Store().Close()
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	goal := domain.Goal{ID: "legacy", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(2000, "EUR"), TargetDate: time.Now().AddDate(0, 0, -1)}
	if err := app.Store().PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	savedGoal := app.Goals()[0]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.Fragment(ui.CreateElement(dashboardGoalCoverageFixture, app), ui.CreateElement(goalStatesWidget, app)))
	f.Stabilize()
	if !strings.Contains(f.Text(), uistate.T("calculation.unavailable")) || strings.Contains(f.Text(), "100%") {
		t.Fatalf("unknown saved FX became a valid featured percentage: %s", f.Text())
	}
	if byTestID(f, "p", "goal-states-unavailable") == nil || !strings.Contains(byTestID(f, "button", "goal-states-current").Text(), "1") || strings.Contains(byTestID(f, "button", "goal-states-completed").Text(), "1") {
		t.Fatalf("unknown overdue coverage became completed or missed: %s", f.Text())
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 0.5}}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(f.Text(), "100%") || strings.Contains(f.Text(), uistate.T("dashboard.goalUnavailable")) || byTestID(f, "p", "goal-states-unavailable") != nil || !strings.Contains(byTestID(f, "button", "goal-states-completed").Text(), "1") {
		t.Fatalf("settings revision retained unavailable coverage: %s", f.Text())
	}
	if !reflect.DeepEqual(app.Goals()[0], savedGoal) {
		t.Fatal("dashboard evaluation persisted converted money")
	}
}

func TestBudgetLinkedGoalNeedConvertsReservationsAndRejectsUnknownFX(t *testing.T) {
	app := bud2TestApp(t)
	defer app.Store().Close()
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	goal := domain.Goal{ID: "legacy", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetDate: now.AddDate(0, 2, 0),
		TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(100, "USD"),
		Allocations: []domain.GoalAllocation{{AccountID: "foreign", Amount: money.New(400, "EUR")}}}
	if err := app.Store().PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	budget := domain.Budget{ID: "budget", Period: domain.PeriodMonthly, TargetKind: domain.TargetByDate, LinkedGoalID: goal.ID,
		TargetAmount: money.New(1000, "USD"), TargetDate: goal.TargetDate}
	status := budgeting.Status{Remaining: money.Zero("USD")}
	need, err := budgetTargetNeed(app, budget, status, now)
	if err != nil || need.Needed.Amount != 50 || need.Needed.Currency != "USD" {
		t.Fatalf("goal pace did not use converted remaining money: %+v, %v", need, err)
	}
	if !reflect.DeepEqual(app.Goals()[0], goal) {
		t.Fatal("linked target preview changed stored allocations")
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	if _, err := budgetTargetNeed(app, budget, status, now); err == nil {
		t.Fatal("missing reservation FX became a known zero or fallback target pace")
	}
	if err := app.PutSettings(store.Settings{BaseCurrency: "USD", FXRates: map[string]float64{"EUR": 3}}); err != nil {
		t.Fatal(err)
	}
	need, err = budgetTargetNeed(app, budget, status, now)
	if err != nil || need.Needed.Amount != 0 {
		t.Fatalf("confirmed complete goal fell back to the inline budget target: %+v, %v", need, err)
	}
}
