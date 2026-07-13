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

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ContributeResult carries the outcome of a ContributeToGoal call so callers
// can surface appropriate toasts / prompts without re-reading state.
type ContributeResult struct {
	// BecameComplete is true when this contribution pushed the goal to 100%.
	BecameComplete bool
	// TransactionID is non-empty when a ledger transaction was posted (the
	// optional "also move money" path).  Empty when memo-only.
	TransactionID string
}

// ContributeToGoal adds amt to g.CurrentAmount and persists the goal.
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
// amt must be positive and share the goal's currency; the caller should validate
// before calling (the UI does this already).
func (a *App) ContributeToGoal(g domain.Goal, amt money.Money, postLedger bool) (ContributeResult, error) {
	if amt.Amount <= 0 {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: amount must be positive")
	}
	if amt.Currency != g.CurrentAmount.Currency && g.CurrentAmount.Currency != "" {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: currency mismatch: %q vs %q", amt.Currency, g.CurrentAmount.Currency)
	}

	wasComplete := g.CurrentAmount.Amount >= g.TargetAmount.Amount && g.TargetAmount.Amount > 0

	g.CurrentAmount = money.New(g.CurrentAmount.Amount+amt.Amount, amt.Currency)
	becameComplete := !wasComplete && g.CurrentAmount.Amount >= g.TargetAmount.Amount && g.TargetAmount.Amount > 0

	// Post the ledger entry (if requested) BEFORE saving the goal, so the
	// contribution log can record the transaction id and "undo contribution" can
	// later remove both the progress and the ledger entry in one action.
	var txnID string
	var postErr error
	if postLedger && g.AccountID != "" {
		// A debit (expense-signed, negative amount) against the linked account. No
		// CategoryID so it doesn't distort budget rollups — the user can categorise
		// it manually if they wish.
		txn := domain.Transaction{
			ID:        id.New(),
			AccountID: g.AccountID,
			Date:      time.Now(),
			Payee:     fmt.Sprintf("Goal contribution — %s", g.Name),
			Desc:      fmt.Sprintf("Contribution to savings goal %q", g.Name),
			Amount:    money.New(-amt.Amount, amt.Currency), // debit
			Source:    domain.TxnSourceManual,               // a deliberate user contribution
		}
		if err := a.PutTransaction(txn); err != nil {
			postErr = fmt.Errorf("appstate: contribute: post ledger: %w", err)
		} else {
			txnID = txn.ID
			a.log.Info("goal contribution ledger entry posted", "goal", g.ID, "txn", txnID)
		}
	}

	g = g.RecordContribution(domain.GoalContribution{Amount: amt, TxnID: txnID, At: time.Now()})
	g.LastReviewedAt = time.Now() // a contribution counts as touching the goal (review freshness)
	if err := a.PutGoal(g); err != nil {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: save goal: %w", err)
	}
	if postErr != nil {
		// The goal (with its bumped amount) is saved; surface the ledger error but
		// don't roll back — the contribution still counts, just without a txn.
		return ContributeResult{BecameComplete: becameComplete}, postErr
	}

	return ContributeResult{BecameComplete: becameComplete, TransactionID: txnID}, nil
}

// UndoLastContribution reverses the most recent contribution to a goal: it drops
// the entry from the log, subtracts its amount from CurrentAmount (floored at
// zero), and deletes the linked ledger transaction if one was posted. It returns
// the undone amount and ok=false when there is nothing to undo.
func (a *App) UndoLastContribution(g domain.Goal) (money.Money, bool, error) {
	updated, last, ok := g.PopLastContribution()
	if !ok {
		return money.Money{}, false, nil
	}
	newAmt := updated.CurrentAmount.Amount - last.Amount.Amount
	if newAmt < 0 {
		newAmt = 0
	}
	updated.CurrentAmount = money.New(newAmt, updated.CurrentAmount.Currency)
	if err := a.PutGoal(updated); err != nil {
		return money.Money{}, false, fmt.Errorf("appstate: undo contribution: save goal: %w", err)
	}
	// Remove the ledger entry this contribution posted, if it still exists. A
	// failure here is non-fatal: the goal progress is already reversed, and the
	// user can delete a stray transaction manually.
	if last.TxnID != "" {
		if err := a.DeleteTransaction(last.TxnID); err != nil {
			a.log.Warn("undo contribution: delete ledger txn failed", "goal", g.ID, "txn", last.TxnID, "err", err)
		}
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
