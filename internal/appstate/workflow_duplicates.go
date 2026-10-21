// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/workflow"
)

// DuplicatePayYourselfFirstError reports a saved PYF workflow on the same
// transfer route and cadence. ExistingEnabled lets the screen preserve its
// enabled/off-specific message without duplicating identity checks.
type DuplicatePayYourselfFirstError struct {
	ExistingEnabled bool
}

func (e *DuplicatePayYourselfFirstError) Error() string {
	return "appstate: pay-yourself-first: a transfer on this route and cadence already exists"
}

// checkPayYourselfFirstDuplicate enforces the PYF identity at the appstate
// write boundary, including for callers other than the quick-start screen.
// The store read and subsequent write are separate operations; this prevents
// duplicates already present at save time but is not an atomic guarantee
// against concurrent writers.
func (a *App) checkPayYourselfFirstDuplicate(candidate workflow.Workflow) error {
	if !workflow.IsPayYourselfFirst(candidate) {
		return nil
	}
	existing, err := a.store.ListWorkflows()
	if err != nil {
		return fmt.Errorf("appstate: check pay-yourself-first duplicates: %w", err)
	}
	duplicate, ok := workflow.FindPayYourselfFirstDuplicate(candidate, existing)
	if !ok {
		return nil
	}
	return &DuplicatePayYourselfFirstError{ExistingEnabled: duplicate.Enabled}
}
