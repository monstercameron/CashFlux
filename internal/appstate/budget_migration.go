// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"reflect"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// MigrateBudgetPeriodsAtomic is the only budget write API that deliberately
// accepts replacement limit history and rollover starts from the caller. Use it
// only to migrate date-keyed state after a calendar preference change. It refuses
// creates and changes to the standing plan or other metadata, validates all dates
// against the destination grid, and commits the complete migration transactionally.
// Ordinary edits must use PutBudgetAt or PutBudgetsAtomicAt instead.
func (a *App) MigrateBudgetPeriodsAtomic(writes []domain.Budget, weekStart time.Weekday, payAnchor time.Time) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if len(writes) == 0 {
		return fmt.Errorf("appstate: budget migration is empty")
	}
	if weekStart < time.Sunday || weekStart > time.Saturday {
		return fmt.Errorf("appstate: invalid migration week start")
	}
	current := make(map[string]domain.Budget)
	for _, b := range a.Budgets() {
		current[b.ID] = b
	}
	seen := make(map[string]bool, len(writes))
	for _, b := range writes {
		old, ok := current[b.ID]
		if !ok || seen[b.ID] {
			return fmt.Errorf("appstate: missing or duplicate migration budget %q", b.ID)
		}
		seen[b.ID] = true
		// Only period state may move; a migration cannot bypass normal edits.
		plan := b
		plan.LimitHistory, plan.PeriodBoosts, plan.PeriodNotes, plan.RolloverStart = old.LimitHistory, old.PeriodBoosts, old.PeriodNotes, old.RolloverStart
		if !reflect.DeepEqual(plan, old) {
			return fmt.Errorf("appstate: budget %q migration changes the standing plan", b.ID)
		}
		if issues := validate.ValidateBudget(b); !issues.OK() {
			return fmt.Errorf("appstate: validate migrated budget %q: %w", b.ID, issues)
		}
		if err := a.validateCustom("budget", b.Custom); err != nil {
			return fmt.Errorf("appstate: validate migrated budget %q custom fields: %w", b.ID, err)
		}
		checkDate := func(key string) error {
			date, err := time.Parse(time.DateOnly, key)
			if err != nil {
				return fmt.Errorf("appstate: budget %q migration date: %w", b.ID, err)
			}
			if budgeting.PeriodKeyWithWeekStart(b.Period, date, payAnchor, weekStart) != key {
				return fmt.Errorf("appstate: budget %q migration date %q is off the new grid", b.ID, key)
			}
			return nil
		}
		for key, amount := range b.LimitHistory {
			if amount <= 0 {
				return fmt.Errorf("appstate: budget %q migration history must be positive", b.ID)
			}
			if err := checkDate(key); err != nil {
				return err
			}
		}
		for key := range b.PeriodBoosts {
			if err := checkDate(key); err != nil {
				return err
			}
		}
		for key := range b.PeriodNotes {
			if err := checkDate(key); err != nil {
				return err
			}
		}
		if !b.RolloverStart.IsZero() {
			if err := checkDate(b.RolloverStart.Format(time.DateOnly)); err != nil {
				return err
			}
		}
	}
	if err := a.store.PutBudgetsAtomic(writes); err != nil {
		return fmt.Errorf("appstate: commit budget period migration: %w", err)
	}
	return nil
}
