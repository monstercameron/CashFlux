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
	"regexp"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/savings"
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
		// {period} resolves to the CURRENT period key at each run (see
		// stampDedupePeriod), scoping the guard to one transfer per period —
		// a creation-frozen stamp would match its own first run's record
		// forever and silently block every transfer after the first.
		DedupeKey: "pyf:" + wfID + ":{period}",
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

// CreatePayYourselfFirstWorkflow builds and persists a pay-yourself-first (PYF)
// scheduled workflow with explicitly supplied accounts. Unlike
// CreateWorkflowFromGoal (which auto-picks the funding account from a goal),
// this variant is driven from the UI where the user has chosen both the source
// (from) and destination (to) accounts and the cadence.
//
// The workflow is named "Pay yourself first → <to-account name>" and the
// DedupeKey follows the same "pyf:<workflowID>:<YYYY-MM>" convention so the
// same period never transfers twice even if the scheduled runner fires more than
// once within a month.
func (a *App) CreatePayYourselfFirstWorkflow(fromID, toID string, amountMinor int64, cadence domain.RecurringCadence) (workflow.Workflow, error) {
	if fromID == "" {
		return workflow.Workflow{}, fmt.Errorf("appstate: pay-yourself-first: source account is required")
	}
	if toID == "" {
		return workflow.Workflow{}, fmt.Errorf("appstate: pay-yourself-first: destination account is required")
	}
	if fromID == toID {
		return workflow.Workflow{}, fmt.Errorf("appstate: pay-yourself-first: source and destination must be different accounts")
	}
	if amountMinor <= 0 {
		return workflow.Workflow{}, fmt.Errorf("appstate: pay-yourself-first: amount must be positive, got %d", amountMinor)
	}

	// Resolve the destination account name for the workflow label.
	toName := toID
	for _, ac := range a.Accounts() {
		if ac.ID == toID {
			toName = ac.Name
			break
		}
	}

	wfID := id.New()
	now := a.clock()
	nextRun := firstOfNextMonth(now)
	if cadence == domain.CadenceWeekly {
		nextRun = now.AddDate(0, 0, 7-int(now.Weekday()))
	}

	act := workflow.Action{
		Kind:                  workflow.ActionTransfer,
		TransferFromAccountID: fromID,
		TransferToAccountID:   toID,
		TransferAmount:        amountMinor,
		// {period} resolves to the CURRENT period key at each run (runWorkflow
		// stamps it) so the guard scopes to one transfer per period. A key
		// frozen at creation time would match its own first run's record
		// forever and silently block every transfer after the first.
		DedupeKey: "pyf:" + wfID + ":{period}",
	}
	if err := workflow.ValidateTransferAction(act, workflow.TriggerScheduled); err != nil {
		return workflow.Workflow{}, fmt.Errorf("appstate: pay-yourself-first: %w", err)
	}

	wf := workflow.Workflow{
		ID:      wfID,
		Name:    "Pay yourself first → " + toName,
		Enabled: true,
		Trigger: workflow.Trigger{
			Kind:    workflow.TriggerScheduled,
			Cadence: cadence,
			NextRun: nextRun,
		},
		Actions: []workflow.Action{act},
	}

	if err := a.PutWorkflow(wf); err != nil {
		return workflow.Workflow{}, fmt.Errorf("appstate: pay-yourself-first: save workflow: %w", err)
	}
	a.log.Info("pay-yourself-first workflow created", "workflow", wf.ID, "from", fromID, "to", toID, "amount", amountMinor, "cadence", cadence)
	return wf, nil
}

// legacyPeriodSuffix matches a DedupeKey whose trailing segment is a frozen
// creation-time month stamp (":YYYY-MM") — the pre-{period} format.
var legacyPeriodSuffix = regexp.MustCompile(`:\d{4}-\d{2}$`)

// stampDedupePeriod resolves a transfer DedupeKey to the current period key:
// the "{period}" placeholder becomes the cadence-appropriate key (ISO week for
// weekly cadence, calendar month otherwise), and a legacy key ending in a
// frozen ":YYYY-MM" stamp is re-stamped the same way — repairing old
// pay-yourself-first workflows that would otherwise transfer once and then
// match their own first run's key forever.
func stampDedupePeriod(key string, cadence domain.RecurringCadence, now time.Time) string {
	period := "monthly"
	if cadence == domain.CadenceWeekly {
		period = "weekly"
	}
	pk := savings.PeriodKey(now, period)
	if strings.Contains(key, "{period}") {
		return strings.ReplaceAll(key, "{period}", pk)
	}
	if legacyPeriodSuffix.MatchString(key) {
		return legacyPeriodSuffix.ReplaceAllString(key, ":"+pk)
	}
	return key
}

// transferSummary renders a transfer effect as money + account names ("Move
// 250.00 USD from Checking to Savings") instead of the engine's raw
// minor-units-and-ids line, for dry-run previews and the run log.
func (a *App) transferSummary(e workflow.Effect) string {
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)
	name := func(id string) string {
		for _, ac := range a.Accounts() {
			if ac.ID == id {
				return ac.Name
			}
		}
		return id
	}
	return fmt.Sprintf("Move %s %s from %s to %s",
		money.FormatMinor(e.TransferAmount, dec), base,
		name(e.TransferFromAccountID), name(e.TransferToAccountID))
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
