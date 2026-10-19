// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/validate"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// PutBudgetsAtomic validates and persists a set of existing budget updates as
// one SQLite transaction. No budget changes are visible if validation or the
// underlying transaction fails.
func (a *App) PutBudgetsAtomic(writes []domain.Budget) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	if len(writes) == 0 {
		return fmt.Errorf("appstate: budget batch is empty")
	}
	wasOver := make(map[string]bool, len(writes))
	seen := make(map[string]bool, len(writes))
	current := make(map[string]domain.Budget, len(writes))
	for _, budget := range a.Budgets() {
		current[budget.ID] = budget
	}
	for _, budget := range writes {
		if seen[budget.ID] {
			return fmt.Errorf("appstate: budget %q appears more than once in batch", budget.ID)
		}
		seen[budget.ID] = true
		if is := validate.ValidateBudget(budget); !is.OK() {
			return fmt.Errorf("appstate: validate budget %q: %w", budget.ID, is)
		}
		if err := a.validateCustom("budget", budget.Custom); err != nil {
			return fmt.Errorf("appstate: validate custom fields for budget %q: %w", budget.ID, err)
		}
		if old, ok := current[budget.ID]; ok {
			wasOver[budget.ID] = a.isBudgetOver(old)
		} else {
			return fmt.Errorf("appstate: budget %q not found", budget.ID)
		}
	}
	if err := a.store.PutBudgetsAtomic(writes); err != nil {
		return fmt.Errorf("appstate: commit budget batch: %w", err)
	}
	for _, budget := range writes {
		a.log.Info("budget saved", "id", budget.ID)
		if !wasOver[budget.ID] && !a.triggersSuspended && a.isBudgetOver(budget) {
			a.RunTriggered(workflow.TriggerBudgetExceeded, nil)
		}
	}
	return nil
}
