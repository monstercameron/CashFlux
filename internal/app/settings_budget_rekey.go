// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

func budgetPeriodPreferencesChanged(old, next prefs.Prefs) bool {
	return old.WeekStartWeekday() != next.WeekStartWeekday() || old.PayCycleAnchor != next.PayCycleAnchor
}

func rekeyBudgetPeriodPreferences(app *appstate.App, old, next prefs.Prefs, ref time.Time) error {
	if app == nil || !budgetPeriodPreferencesChanged(old, next) {
		return nil
	}
	writes, err := rekeyBudgetPeriodData(app.Budgets(), old, next)
	if err != nil {
		return err
	}
	if len(writes) == 0 {
		return nil
	}
	if err := app.MigrateBudgetPeriodsAtomic(writes, next.WeekStartWeekday(), budgetPeriodPreferenceAnchor(next)); err != nil {
		return fmt.Errorf("save re-keyed budget periods: %w", err)
	}
	return nil
}

func budgetPeriodPreferenceAnchor(p prefs.Prefs) time.Time {
	if p.PayCycleAnchor == "" {
		return time.Time{}
	}
	anchor, err := time.Parse(time.DateOnly, p.PayCycleAnchor)
	if err != nil {
		return time.Time{}
	}
	return anchor
}

// rekeyBudgetPeriodData moves date-keyed budget state to the period grid described by next.
func rekeyBudgetPeriodData(budgets []domain.Budget, old, next prefs.Prefs) ([]domain.Budget, error) {
	oldWeek, nextWeek := old.WeekStartWeekday(), next.WeekStartWeekday()
	oldAnchor, nextAnchor := budgetPeriodPreferenceAnchor(old), budgetPeriodPreferenceAnchor(next)
	writes := make([]domain.Budget, 0, len(budgets))
	for _, budget := range budgets {
		updated := budget
		var err error
		if updated.PeriodBoosts, err = rekeyIntPeriodMap(budget.ID, budget.Period, budget.PeriodBoosts, nextWeek, nextAnchor); err != nil {
			return nil, err
		}
		if updated.LimitHistory, err = rekeyIntPeriodMap(budget.ID, budget.Period, budget.LimitHistory, nextWeek, nextAnchor); err != nil {
			return nil, err
		}
		if updated.PeriodNotes, err = rekeyStringPeriodMap(budget.ID, budget.Period, budget.PeriodNotes, nextWeek, nextAnchor); err != nil {
			return nil, err
		}
		if !budget.RolloverStart.IsZero() {
			updated.RolloverStart, err = rekeyPeriodDate(budget.ID, budget.Period, budget.RolloverStart, oldWeek, nextWeek, oldAnchor, nextAnchor)
			if err != nil {
				return nil, err
			}
		}
		writes = append(writes, updated)
	}
	return writes, nil
}

func rekeyIntPeriodMap(budgetID string, period domain.Period, values map[string]int64, nextWeek time.Weekday, nextAnchor time.Time) (map[string]int64, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]int64, len(values))
	for key, value := range values {
		date, err := time.Parse(time.DateOnly, key)
		if err != nil {
			return nil, fmt.Errorf("budget %q period key %q: %w", budgetID, key, err)
		}
		newKey := budgeting.PeriodKeyWithWeekStart(period, date, nextAnchor, nextWeek)
		if _, exists := out[newKey]; exists {
			return nil, fmt.Errorf("budget %q period keys collide at %s", budgetID, newKey)
		}
		out[newKey] = value
	}
	return out, nil
}

func rekeyStringPeriodMap(budgetID string, period domain.Period, values map[string]string, nextWeek time.Weekday, nextAnchor time.Time) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		date, err := time.Parse(time.DateOnly, key)
		if err != nil {
			return nil, fmt.Errorf("budget %q period note key %q: %w", budgetID, key, err)
		}
		newKey := budgeting.PeriodKeyWithWeekStart(period, date, nextAnchor, nextWeek)
		if _, exists := out[newKey]; exists {
			return nil, fmt.Errorf("budget %q period note keys collide at %s", budgetID, newKey)
		}
		out[newKey] = value
	}
	return out, nil
}

func rekeyPeriodDate(budgetID string, period domain.Period, date time.Time, oldWeek, nextWeek time.Weekday, oldAnchor, nextAnchor time.Time) (time.Time, error) {
	oldStart := budgeting.PeriodKeyWithWeekStart(period, date, oldAnchor, oldWeek)
	oldDate, err := time.Parse(time.DateOnly, oldStart)
	if err != nil {
		return time.Time{}, fmt.Errorf("budget %q rollover start: %w", budgetID, err)
	}
	newStart := budgeting.PeriodKeyWithWeekStart(period, oldDate, nextAnchor, nextWeek)
	newDate, err := time.Parse(time.DateOnly, newStart)
	if err != nil {
		return time.Time{}, fmt.Errorf("budget %q new rollover start: %w", budgetID, err)
	}
	return newDate, nil
}
