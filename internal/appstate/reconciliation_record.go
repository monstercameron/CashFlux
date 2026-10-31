// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// RecordOutcome says what recording a reconciliation did beyond saving it.
type RecordOutcome struct {
	// Retired is the number of provisional checkpoints the statement superseded and removed.
	Retired int
}

// RecordReconciliation saves a completed reconciliation to the account's history and stamps the
// balance as of the statement. It also retires the provisional balance checkpoints the statement
// supersedes, because the statement's own rows now carry that balance. A normal and a forced
// reconciliation both go through here, so both retire the same checkpoints.
//
// The account update and every checkpoint removal commit together: on error nothing changed.
func (a *App) RecordReconciliation(accountID string, ev domain.Reconciliation) (RecordOutcome, error) {
	if err := a.roleGuard(); err != nil {
		return RecordOutcome{}, err
	}
	account, ok, err := a.store.GetAccount(accountID)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: read account: %w", err)
	}
	if !ok {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: account %q not found", accountID)
	}
	updated := account
	updated.Reconciliations = reconcile.Record(account.Reconciliations, ev)
	updated.BalanceAsOf = a.clock()
	if is := validate.ValidateAccount(updated); !is.OK() {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: save account: %w", is)
	}
	if err := a.validateCustom("account", updated.Custom); err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: save account: %w", err)
	}
	changes, retired, err := a.transactionDeleteChanges(provisional.SupersededBy(accountID, a.Transactions(), ev.Through()))
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: plan checkpoint removal: %w", err)
	}
	before, err := json.Marshal(account)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: encode account: %w", err)
	}
	after, err := json.Marshal(updated)
	if err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: encode updated account: %w", err)
	}
	changes = append(changes, store.RecordChange{Table: "accounts", ID: accountID, Before: before, After: after})
	if err := a.store.ReplayChanges(changes); err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: commit: %w", err)
	}
	a.log.Info("reconciliation recorded", "account", accountID, "retired", len(retired))
	if len(retired) > 0 {
		a.afterTransactionsDeleted(changes, retired)
	}
	return RecordOutcome{Retired: len(retired)}, nil
}
