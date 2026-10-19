// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"time"
)

func budgetPeriodPreferencesChanged(old, next prefs.Prefs) bool {
	return old.WeekStartWeekday() != next.WeekStartWeekday() || old.PayCycleAnchor != next.PayCycleAnchor
}

// rekeyBudgetPeriodPreferences is the compatibility migration adapter. Live
// writers use uistate.SetPrefs, which saves the grid and amounts together.
func rekeyBudgetPeriodPreferences(app *appstate.App, old, next prefs.Prefs, _ time.Time) error {
	if app == nil || !budgetPeriodPreferencesChanged(old, next) {
		return nil
	}
	writes, err := rekeyBudgetPeriodData(app.Budgets(), old, next)
	if err != nil || len(writes) == 0 {
		return err
	}
	anchor, err := time.Parse(time.DateOnly, next.PayCycleAnchor)
	if next.PayCycleAnchor != "" && err != nil {
		return err
	}
	return app.MigrateBudgetPeriodsAtomic(writes, next.WeekStartWeekday(), anchor)
}

func rekeyBudgetPeriodData(budgets []domain.Budget, old, next prefs.Prefs) ([]domain.Budget, error) {
	return appstate.RekeyBudgetPeriodData(budgets, old, next)
}
