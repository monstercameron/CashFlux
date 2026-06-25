// SPDX-License-Identifier: MIT

// Package appstate — savings automation helpers.
//
// This file holds CreateWorkflowFromGoal, which builds and persists a
// pay-yourself-first (PYF) scheduled workflow for a goal. It lives here
// (rather than in the workflow package) because it needs to read accounts and
// goals from the live store to resolve the funding account.
//
// No syscall/js dependency; the file is unit-tested on native Go.
package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// CreateWorkflowFromGoal builds and persists a pay-yourself-first scheduled
// workflow for the given goal. The workflow triggers monthly and transfers
// monthlyAmount (in the goal's currency minor units) from a deterministically
// chosen funding account to the goal's linked account (goal.AccountID).
//
// Funding-account selection: the first non-archived asset account (sorted by
// store order — stable and deterministic) that is not the goal's own destination
// account is used. Checking and debit accounts are preferred over savings and
// other asset types, so money moves from a liquid spending account rather than
// from a separate savings bucket. If no eligible funding account exists,
// CreateWorkflowFromGoal returns an error.
//
// The DedupeKey follows the convention "pyf:<workflowID>:<YYYY-MM>" so the same
// monthly period never transfers twice even if the scheduled runner fires more
// than once. ValidateTransferAction is called before PutWorkflow.
func (a *App) CreateWorkflowFromGoal(goalID string, monthlyAmount int64) (workflow.Workflow, error) {
	// Resolve the goal.
	var goal domain.Goal
	var found bool
	for _, g := range a.Goals() {
		if g.ID == goalID {
			goal, found = g, true
			break
		}
	}
	if !found {
		return workflow.Workflow{}, fmt.Errorf("appstate: automate-goal: goal %q not found", goalID)
	}
	if goal.AccountID == "" {
		return workflow.Workflow{}, fmt.Errorf("appstate: automate-goal: goal %q has no linked account — link an account before automating", goalID)
	}
	if monthlyAmount <= 0 {
		return workflow.Workflow{}, fmt.Errorf("appstate: automate-goal: monthly amount must be positive, got %d", monthlyAmount)
	}

	// Choose the funding account: first non-archived asset account that is not
	// the goal's own destination account, preferring checking/debit types.
	fundingID := pickFundingAccount(a.Accounts(), goal.AccountID)
	if fundingID == "" {
		return workflow.Workflow{}, fmt.Errorf("appstate: automate-goal: no eligible funding account found (need at least one non-archived asset account that is not the goal's account)")
	}

	wfID := id.New()
	now := a.clock()
	// First run on the first of next month.
	nextRun := firstOfNextMonth(now)

	act := workflow.Action{
		Kind:                  workflow.ActionTransfer,
		TransferFromAccountID: fundingID,
		TransferToAccountID:   goal.AccountID,
		TransferAmount:        monthlyAmount,
		// DedupeKey template: the period key is appended at run time by the apply
		// layer using the workflow ID prefix. Here we embed the wfID so the key
		// space is scoped per-workflow; the period suffix prevents double-execution
		// within the same calendar month.
		DedupeKey: "pyf:" + wfID + ":" + now.Format("2006-01"),
	}
	if err := workflow.ValidateTransferAction(act, workflow.TriggerScheduled); err != nil {
		return workflow.Workflow{}, fmt.Errorf("appstate: automate-goal: %w", err)
	}

	wf := workflow.Workflow{
		ID:      wfID,
		Name:    "Auto-contribute to " + goal.Name,
		Enabled: true,
		Trigger: workflow.Trigger{
			Kind:    workflow.TriggerScheduled,
			Cadence: domain.CadenceMonthly,
			NextRun: nextRun,
		},
		Actions: []workflow.Action{act},
	}

	if err := a.PutWorkflow(wf); err != nil {
		return workflow.Workflow{}, fmt.Errorf("appstate: automate-goal: save workflow: %w", err)
	}
	a.log.Info("pay-yourself-first workflow created", "workflow", wf.ID, "goal", goalID, "monthly", monthlyAmount)
	return wf, nil
}

// pickFundingAccount returns the ID of the best funding account: the first
// non-archived asset account (not equal to excludeID), giving preference to
// checking and debit account types (most liquid) over savings and other types.
func pickFundingAccount(accounts []domain.Account, excludeID string) string {
	// Two-pass: first look for a preferred (checking/debit) account, then accept
	// any asset account if no preferred one exists.
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassAsset || a.ID == excludeID {
			continue
		}
		if a.Type == domain.TypeChecking || a.Type == domain.TypeDebit {
			return a.ID
		}
	}
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassAsset || a.ID == excludeID {
			continue
		}
		return a.ID
	}
	return ""
}

// firstOfNextMonth returns midnight UTC on the first day of the month after t.
func firstOfNextMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	m++
	if m > 12 {
		m, y = 1, y+1
	}
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}
