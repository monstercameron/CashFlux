// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// MigrateBudgetPeriodsAtomic is the only budget write API that deliberately
// accepts replacement limit history and rollover starts from the caller. Use it
// only to migrate date-keyed state after a calendar preference change. It refuses
// creates and changes to the standing plan or other metadata, validates all dates
// against the destination grid, and commits the complete migration transactionally.
// Ordinary edits must use PutBudgetAt or PutBudgetsAtomicAt instead.
func (a *App) MigrateBudgetPeriodsAtomic(writes []domain.Budget, weekStart time.Weekday, payAnchor time.Time) error {
	return a.migrateBudgetPeriods(writes, weekStart, payAnchor, nil)
}

func (a *App) migrateBudgetPeriods(writes []domain.Budget, weekStart time.Weekday, payAnchor time.Time, next *prefs.Prefs) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if len(writes) == 0 && next == nil {
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
	changes := make([]store.RecordChange, 0, len(writes)+1)
	for _, b := range writes {
		old, ok := current[b.ID]
		if !ok || seen[b.ID] {
			return fmt.Errorf("appstate: missing or duplicate migration budget %q", b.ID)
		}
		seen[b.ID] = true
		// Only period state may move; a migration cannot bypass normal edits.
		plan := b
		plan.LimitHistory, plan.PeriodBoosts, plan.PeriodNotes, plan.RolloverStart = old.LimitHistory, old.PeriodBoosts, old.PeriodNotes, old.RolloverStart
		plan.PeriodTopups = old.PeriodTopups
		if plan.RecurringCover != nil && old.RecurringCover != nil {
			rc := *plan.RecurringCover
			rc.LastAppliedPeriod = old.RecurringCover.LastAppliedPeriod
			plan.RecurringCover = &rc
		}
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
		for key := range b.PeriodTopups {
			if err := checkDate(key); err != nil {
				return err
			}
		}
		if b.RecurringCover != nil && b.RecurringCover.LastAppliedPeriod != "" {
			if err := checkDate(b.RecurringCover.LastAppliedPeriod); err != nil {
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
		before, err := json.Marshal(old)
		if err != nil {
			return fmt.Errorf("encode prior budget migration: %w", err)
		}
		after, err := json.Marshal(b)
		if err != nil {
			return fmt.Errorf("encode budget migration: %w", err)
		}
		changes = append(changes, store.RecordChange{Table: "budgets", ID: b.ID, Before: before, After: after})
	}
	if next != nil {
		raw, exists, err := a.store.GetSettingKV("cashflux:prefs")
		if err != nil {
			return fmt.Errorf("read migration preferences: %w", err)
		}
		var before json.RawMessage
		if exists {
			before, _ = json.Marshal(raw)
		}
		encoded, err := json.Marshal(*next)
		if err != nil {
			return fmt.Errorf("encode migration preferences: %w", err)
		}
		after, _ := json.Marshal(string(encoded))
		changes = append(changes, store.RecordChange{Table: "settingskv", ID: "cashflux:prefs", Before: before, After: after})
	}
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("appstate: commit budget period migration: %w", err)
	}
	return nil
}

// SaveBudgetPeriodPreferences is the migrate-then-save boundary for every
// calendar preference writer. Budgets and preferences enter the change journal
// in one SQL transaction; a failure cannot leave the grid and its amounts apart.
// The UI must await its normal durable boundary before acknowledging success.
func (a *App) SaveBudgetPeriodPreferences(next prefs.Prefs) error {
	next = next.Normalize()
	old, _, err := a.budgetPreferences()
	if err != nil {
		return err
	}
	anchor, err := budgetPreferenceAnchor(next)
	if err != nil {
		return err
	}
	if old.WeekStartWeekday() == next.WeekStartWeekday() && old.PayCycleAnchor == next.PayCycleAnchor {
		encoded, err := json.Marshal(next)
		if err != nil {
			return fmt.Errorf("encode budget preferences: %w", err)
		}
		return a.store.SetSettingKV("cashflux:prefs", string(encoded))
	}
	writes, err := RekeyBudgetPeriodData(a.Budgets(), old, next)
	if err != nil {
		return err
	}
	return a.migrateBudgetPeriods(writes, next.WeekStartWeekday(), anchor, &next)
}

func budgetPreferenceAnchor(p prefs.Prefs) (time.Time, error) {
	if p.PayCycleAnchor == "" {
		return time.Time{}, nil
	}
	anchor, err := time.Parse(time.DateOnly, p.PayCycleAnchor)
	if err != nil {
		return time.Time{}, fmt.Errorf("budget pay-cycle anchor: %w", err)
	}
	return anchor, nil
}

// RekeyBudgetPeriodData maps old periods by their midpoint, preserving amounts,
// notes, funding history and recurring-cover replay stamps on the new grid.
// Legacy boosts are retained exactly and are not assigned invented provenance.
// It returns a new slice and maps; a collision refuses the whole migration.
func RekeyBudgetPeriodData(budgets []domain.Budget, old, next prefs.Prefs) ([]domain.Budget, error) {
	oldAnchor, err := budgetPreferenceAnchor(old)
	if err != nil {
		return nil, err
	}
	nextAnchor, err := budgetPreferenceAnchor(next)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Budget, 0, len(budgets))
	for _, b := range budgets {
		keyOf := func(key string) (string, error) {
			date, err := time.Parse(time.DateOnly, key)
			if err != nil {
				return "", fmt.Errorf("budget %q period key: %w", b.ID, err)
			}
			start, end := budgeting.PeriodRangeAnchored(b.Period, date, old.WeekStartWeekday(), oldAnchor)
			middle := start.Add(end.Sub(start) / 2)
			return budgeting.PeriodKeyWithWeekStart(b.Period, middle, nextAnchor, next.WeekStartWeekday()), nil
		}
		updated := b
		if updated.PeriodBoosts, err = rekeyBudgetAmounts(b.PeriodBoosts, keyOf, false); err != nil {
			return nil, err
		}
		if updated.PeriodTopups, err = rekeyBudgetAmounts(b.PeriodTopups, keyOf, false); err != nil {
			return nil, err
		}
		if updated.LimitHistory, err = rekeyBudgetAmounts(b.LimitHistory, keyOf, false); err != nil {
			return nil, err
		}
		if b.PeriodNotes != nil {
			updated.PeriodNotes = make(map[string]string, len(b.PeriodNotes))
			for key, value := range b.PeriodNotes {
				nextKey, err := keyOf(key)
				if err != nil {
					return nil, err
				}
				if _, exists := updated.PeriodNotes[nextKey]; exists {
					return nil, fmt.Errorf("budget %q period notes collide at %s", b.ID, nextKey)
				}
				updated.PeriodNotes[nextKey] = value
			}
		}
		if !b.RolloverStart.IsZero() {
			key, err := keyOf(b.RolloverStart.Format(time.DateOnly))
			if err != nil {
				return nil, err
			}
			updated.RolloverStart, _ = time.Parse(time.DateOnly, key)
		}
		if b.RecurringCover != nil && b.RecurringCover.LastAppliedPeriod != "" {
			rc := *b.RecurringCover
			rc.LastAppliedPeriod, err = keyOf(rc.LastAppliedPeriod)
			if err != nil {
				return nil, err
			}
			updated.RecurringCover = &rc
		}
		out = append(out, updated)
	}
	return out, nil
}

func rekeyBudgetAmounts(values map[string]int64, keyOf func(string) (string, error), sumCollisions bool) (map[string]int64, error) {
	if values == nil {
		return nil, nil
	}
	out := make(map[string]int64, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		next, err := keyOf(key)
		if err != nil {
			return nil, err
		}
		if _, exists := out[next]; exists && !sumCollisions {
			return nil, fmt.Errorf("budget periods collide at %s", next)
		}
		out[next], err = money.Add(out[next], values[key])
		if err != nil {
			return nil, fmt.Errorf("re-key budget amount: %w", err)
		}
	}
	return out, nil
}
