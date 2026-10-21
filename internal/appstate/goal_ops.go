// SPDX-License-Identifier: MIT

// Package appstate — goal-specific orchestration methods.
//
// This file holds operations that span goal + transaction state (e.g. the
// optional ledger-posting path for contributions). They live here rather than
// in appstate.go to keep the seam file focused on boilerplate accessors.
package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// ContributeResult carries the outcome of a ContributeToGoal call so callers
// can surface appropriate toasts / prompts without re-reading state.
type ContributeResult struct {
	// SavedProgressKnown makes the before/after saved-only percentages usable.
	// Milestone is the highest known saved-only threshold crossed, or zero. The
	// completion-worded 100% celebration also requires known after-coverage.
	SavedProgressKnown                    bool
	BeforeSavedPercent, AfterSavedPercent int
	Milestone                             int
	// CompletionKnown makes the coverage-derived completion transition usable.
	// Unknown FX or unsafe coverage arithmetic never produces a completion prompt.
	CompletionKnown bool
	// BecameComplete is true when known coverage crossed the goal's target.
	BecameComplete bool
	// TransactionID is non-empty when a ledger transaction was posted (the
	// optional "also move money" path).  Empty when memo-only.
	TransactionID string
}

// ContributeToGoal adds amt to the current stored goal identified by g.ID.
// Captured metadata, saved progress and the primary account are re-read at submit.
//
// When postLedger is true AND the goal has a linked account (g.AccountID ≠ ""),
// it also posts a debit transaction against that account so the contribution is
// reflected in the ledger — the "also move money from <linked account>" path.
// The transaction is a plain expense-signed entry (negative Amount) with a
// descriptive payee so it is reversible: the user can delete it from the
// Transactions screen like any other entry.
//
// When postLedger is false, or the goal has no linked account, the call is
// memo-only: only CurrentAmount is bumped (the historical behaviour).
//
// amt must be positive and share the saved amount's currency. Arithmetic and the
// intended goal write are checked before any mutation. A requested ledger debit
// and the goal commit together; an error leaves both unchanged.
func (a *App) ContributeToGoal(g domain.Goal, amt money.Money, postLedger bool) (ContributeResult, error) {
	latest, err := a.storedContributionGoal(g.ID)
	if err != nil {
		return ContributeResult{}, err
	}
	return a.ContributeToGoalFrom(latest, amt, postLedger, latest.AccountID)
}

// ContributeToGoalFrom is ContributeToGoal with an explicit ledger-post account:
// fromAcctID names which account the debit posts against (a goal linked to several
// accounts lets the user pick), WITHOUT altering the goal's stored primary
// AccountID. An empty fromAcctID (or postLedger=false) is memo-only.
func (a *App) ContributeToGoalFrom(g domain.Goal, amt money.Money, postLedger bool, fromAcctID string) (ContributeResult, error) {
	if err := a.roleGuard(); err != nil {
		return ContributeResult{}, err
	}
	latest, err := a.storedContributionGoal(g.ID)
	if err != nil {
		return ContributeResult{}, err
	}
	g = latest
	if amt.Amount <= 0 {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: amount must be positive")
	}
	if amt.Currency == "" {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: amount currency is required")
	}
	if g.CurrentAmount.Currency == "" && g.CurrentAmount.Amount != 0 {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: saved amount has no currency")
	}
	if amt.Currency != g.CurrentAmount.Currency && g.CurrentAmount.Currency != "" {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: currency mismatch: %q vs %q", amt.Currency, g.CurrentAmount.Currency)
	}

	updatedAmount, err := money.Add(g.CurrentAmount.Amount, amt.Amount)
	if err != nil {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: saved amount: %w", err)
	}
	before := g
	g.CurrentAmount = money.New(updatedAmount, amt.Currency)
	// Validate the intended goal write before optionally posting a debit. Invalid
	// imported currencies/custom fields must not leave a stray ledger entry.
	if issues := validate.ValidateGoal(g); !issues.OK() {
		return ContributeResult{}, issues
	}
	if err := a.validateCustom("goal", g.Custom); err != nil {
		return ContributeResult{}, err
	}
	contributedAt := time.Now()
	g, err = g.RecordContributionChecked(domain.GoalContribution{Amount: amt, At: contributedAt})
	if err != nil {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: record history: %w", err)
	}
	g.LastReviewedAt = contributedAt
	rates := a.goalEvaluationRates()
	beforeCoverage, beforeErr := goals.EvaluateCoverage(before, rates)
	afterCoverage, afterErr := goals.EvaluateCoverage(g, rates)
	result := ContributeResult{SavedProgressKnown: beforeCoverage.SavedKnown && afterCoverage.SavedKnown,
		CompletionKnown: beforeErr == nil && afterErr == nil}
	if result.SavedProgressKnown {
		result.BeforeSavedPercent, result.AfterSavedPercent = beforeCoverage.SavedPercent, afterCoverage.SavedPercent
		result.Milestone = goals.MilestoneCrossed(result.BeforeSavedPercent, result.AfterSavedPercent)
		if result.Milestone == 100 && afterErr != nil {
			result.Milestone = 0
		}
	}
	result.BecameComplete = result.CompletionKnown && !beforeCoverage.Complete && afterCoverage.Complete

	// Prepare the optional debit and attach its ID to the copy-owned, checked
	// contribution log before committing either record.
	var debit *domain.Transaction
	if postLedger && fromAcctID != "" {
		// A debit (expense-signed, negative amount) against the chosen account. No
		// CategoryID so it doesn't distort budget rollups — the user can categorise
		// it manually if they wish.
		txn := domain.Transaction{
			ID:        id.New(),
			AccountID: fromAcctID,
			Date:      contributedAt,
			Payee:     fmt.Sprintf("Goal contribution — %s", g.Name),
			Desc:      fmt.Sprintf("Contribution to savings goal %q", g.Name),
			Amount:    money.New(-amt.Amount, amt.Currency), // debit
			Source:    domain.TxnSourceManual,               // a deliberate user contribution
		}
		debit = &txn
		g.Contributions[len(g.Contributions)-1].TxnID = txn.ID
	}

	if err := a.commitGoalContribution(before, g, debit); err != nil {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: %w", err)
	}
	if debit != nil {
		result.TransactionID = debit.ID
	}
	return result, nil
}

func (a *App) goalEvaluationRates() currency.Rates {
	settings := a.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: settings.FXRates}
}

// goalCompletionForWrite keeps the existing nonfinancial write-trigger contract.
// Financial completion requires known target-currency coverage on both sides.
func goalCompletionForWrite(goal domain.Goal, rates currency.Rates) (complete, known bool) {
	if !goal.IsFinancial() {
		return goal.CurrentAmount.Amount >= goal.TargetAmount.Amount && goal.TargetAmount.Amount > 0, true
	}
	coverage, err := goals.EvaluateCoverage(goal, rates)
	return err == nil && coverage.Complete, err == nil
}

// storedContributionGoal always reads the current store, rather than accepting a
// form's captured goal as the new record. Missing IDs must never be resurrected.
func (a *App) storedContributionGoal(goalID string) (domain.Goal, error) {
	rows, err := a.store.ListGoals()
	if err != nil {
		return domain.Goal{}, fmt.Errorf("appstate: contribution: load goal: %w", err)
	}
	for _, goal := range rows {
		if goal.ID == goalID {
			return goal, nil
		}
	}
	return domain.Goal{}, fmt.Errorf("appstate: contribution: goal %q not found", goalID)
}

// GoalContributionRemaining returns the checked saved-only quick-fill amount for
// the current goal. A legacy unspecified zero uses the target currency; nonzero
// currencies are never relabeled to make an otherwise unsupported write possible.
func (a *App) GoalContributionRemaining(goalID string) (money.Money, error) {
	goal, err := a.storedContributionGoal(goalID)
	if err != nil {
		return money.Money{}, err
	}
	if goal.CurrentAmount.Currency == "" && goal.CurrentAmount.Amount == 0 {
		goal.CurrentAmount = money.Zero(goal.TargetAmount.Currency)
	}
	remaining, err := goals.Remaining(goal)
	if err != nil {
		return money.Money{}, fmt.Errorf("appstate: contribution remaining: %w", err)
	}
	return remaining, nil
}

// UndoLastContribution reverses the most recent contribution to a goal: it drops
// the entry from the log, subtracts its amount from CurrentAmount (floored at
// zero), and deletes the linked ledger transaction if one was posted. It returns
// the undone amount and ok=false when there is nothing to undo. The current stored
// goal is re-read; invalid logged amounts fail before either record is changed.
func (a *App) UndoLastContribution(g domain.Goal) (money.Money, bool, error) {
	if err := a.roleGuard(); err != nil {
		return money.Money{}, false, err
	}
	latest, err := a.storedContributionGoal(g.ID)
	if err != nil {
		return money.Money{}, false, err
	}
	g = latest
	updated, last, ok := g.PopLastContribution()
	if !ok {
		return money.Money{}, false, nil
	}
	if last.Amount.Amount <= 0 {
		return money.Money{}, false, fmt.Errorf("appstate: undo contribution: logged amount must be positive")
	}
	newAmount, err := updated.CurrentAmount.Sub(last.Amount)
	if err != nil {
		return money.Money{}, false, fmt.Errorf("appstate: undo contribution: saved amount: %w", err)
	}
	if newAmount.Amount < 0 {
		newAmount = money.Zero(newAmount.Currency)
	}
	updated.CurrentAmount = newAmount
	if err := a.commitContributionUndo(g, updated, last.TxnID); err != nil {
		return money.Money{}, false, fmt.Errorf("appstate: undo contribution: %w", err)
	}
	return last.Amount, true, nil
}

// MarkGoalReviewed stamps the goal's LastReviewedAt to now, clearing its "review due"
// flag until the ReviewCadence elapses again — the goal's version of "I've looked at
// this". A missing goal id is a silent no-op.
func (a *App) MarkGoalReviewed(goalID string) error {
	for _, g := range a.Goals() {
		if g.ID != goalID {
			continue
		}
		g.LastReviewedAt = time.Now()
		if err := a.PutGoal(g); err != nil {
			return fmt.Errorf("appstate: mark goal reviewed: save goal: %w", err)
		}
		return nil
	}
	return nil
}

// SetGoalAllocations replaces a goal's virtual earmarks and persists it. The amounts are
// trusted (the allocate modal caps each to the account's free balance via
// goals.AvailableToEarmarkMinor); zero-amount entries are dropped so cleared rows don't
// linger. No transaction is posted — earmarks never move money.
func (a *App) SetGoalAllocations(goalID string, allocs []domain.GoalAllocation) error {
	for _, g := range a.Goals() {
		if g.ID != goalID {
			continue
		}
		kept := make([]domain.GoalAllocation, 0, len(allocs))
		for _, al := range allocs {
			if al.AccountID != "" && al.Amount.Amount > 0 {
				kept = append(kept, al)
			}
		}
		g.Allocations = kept
		if err := a.PutGoal(g); err != nil {
			return fmt.Errorf("appstate: set goal allocations: save goal: %w", err)
		}
		return nil
	}
	return nil
}

// ResetGoalToZero clears a goal's saved progress back to zero and empties its
// contribution log. Linked ledger transactions are intentionally left untouched —
// they are real money movements the user manages on the Transactions screen; this
// only resets the goal's tracked figure.
func (a *App) ResetGoalToZero(g domain.Goal) error {
	g.CurrentAmount = money.New(0, g.CurrentAmount.Currency)
	g.Contributions = nil
	if err := a.PutGoal(g); err != nil {
		return fmt.Errorf("appstate: reset goal: save goal: %w", err)
	}
	return nil
}
