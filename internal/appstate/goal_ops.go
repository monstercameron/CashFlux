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
	if err := a.PutGoal(g); err != nil {
		return ContributeResult{}, fmt.Errorf("appstate: contribute: save goal: %w", err)
	}

	becameComplete := !wasComplete && g.CurrentAmount.Amount >= g.TargetAmount.Amount && g.TargetAmount.Amount > 0

	var txnID string
	if postLedger && g.AccountID != "" {
		// Post a debit (expense-signed, negative amount) against the linked
		// account.  The transaction intentionally carries no CategoryID so it
		// does not distort budget rollups — the user can categorise it manually
		// if they wish.
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
			// The goal was already saved; surface the error but don't roll back
			// the goal — the user can delete the bad txn manually.
			return ContributeResult{BecameComplete: becameComplete}, fmt.Errorf("appstate: contribute: post ledger: %w", err)
		}
		txnID = txn.ID
		a.log.Info("goal contribution ledger entry posted", "goal", g.ID, "txn", txnID)
	}

	return ContributeResult{BecameComplete: becameComplete, TransactionID: txnID}, nil
}
