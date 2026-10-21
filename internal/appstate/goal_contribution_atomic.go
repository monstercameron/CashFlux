// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// contributionGoalChange validates the complete intended record, including its
// contribution history, before building a checked transition from the current goal.
func (a *App) contributionGoalChange(before, after domain.Goal) (store.RecordChange, error) {
	if issues := validate.ValidateGoal(after); !issues.OK() {
		return store.RecordChange{}, issues
	}
	if err := a.validateCustom("goal", after.Custom); err != nil {
		return store.RecordChange{}, err
	}
	prior, err := json.Marshal(before)
	if err != nil {
		return store.RecordChange{}, fmt.Errorf("encode prior contribution goal: %w", err)
	}
	next, err := json.Marshal(after)
	if err != nil {
		return store.RecordChange{}, fmt.Errorf("encode contribution goal: %w", err)
	}
	return store.RecordChange{Table: "goals", ID: before.ID, Before: prior, After: next}, nil
}

// commitGoalContribution saves progress and its optional debit together. Effects
// run after commit so workflows and observers always see both records.
func (a *App) commitGoalContribution(before, after domain.Goal, debit *domain.Transaction) error {
	goalChange, err := a.contributionGoalChange(before, after)
	if err != nil {
		return err
	}
	var changes []store.RecordChange
	var txn domain.Transaction
	var overBefore map[string]bool
	if debit != nil {
		txn = *debit
		txn.Date = normalizeTransactionDate(txn.Date)
		txn.NormalizeOwners()
		if issues := validate.ValidateTransaction(txn, a.Accounts()...); !issues.OK() {
			return issues
		}
		if err := a.validateCustom("transaction", txn.Custom); err != nil {
			return err
		}
		data, err := json.Marshal(txn)
		if err != nil {
			return fmt.Errorf("encode contribution debit: %w", err)
		}
		// Before=nil also refuses to overwrite a colliding transaction ID.
		changes = append(changes, store.RecordChange{Table: "transactions", ID: txn.ID, After: data})
		if !a.triggersSuspended {
			overBefore = a.overBudgetIDs()
		}
	}
	changes = append(changes, goalChange)
	rates := a.goalEvaluationRates()
	wasComplete, beforeKnown := goalCompletionForWrite(before, rates)
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("commit contribution: %w", err)
	}
	if debit != nil {
		a.afterTransactionSaved(txn, false, overBefore)
	}
	a.afterGoalSaved(after, wasComplete, beforeKnown, rates)
	return nil
}

// commitContributionUndo keeps the contribution log until its debit and any
// transaction relationships can be removed in the same checked commit.
func (a *App) commitContributionUndo(before, after domain.Goal, transactionID string) error {
	goalChange, err := a.contributionGoalChange(before, after)
	if err != nil {
		return err
	}
	changes := []store.RecordChange{goalChange}
	var transactionChanges []store.RecordChange
	var deleted []string
	if transactionID != "" {
		transactionChanges, deleted, err = a.transactionDeleteChanges([]string{transactionID})
		if err != nil {
			return fmt.Errorf("plan contribution debit deletion: %w", err)
		}
		changes = append(changes, transactionChanges...)
	}
	rates := a.goalEvaluationRates()
	wasComplete, beforeKnown := goalCompletionForWrite(before, rates)
	if err := a.store.ReplayChangesExpected(changes); err != nil {
		return fmt.Errorf("commit contribution undo: %w", err)
	}
	a.afterGoalSaved(after, wasComplete, beforeKnown, rates)
	if len(deleted) > 0 {
		a.afterTransactionsDeleted(transactionChanges, deleted)
	}
	return nil
}
