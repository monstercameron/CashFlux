// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// RecordOutcome says what recording a reconciliation did beyond saving it.
type RecordOutcome struct {
	// Retired is the number of provisional checkpoints the statement superseded and removed.
	Retired int
	// RetireFailures joins any checkpoint that could not be deleted. The reconciliation is recorded
	// regardless, so these stand-ins are still in place.
	RetireFailures error
}

// RecordReconciliation saves a completed reconciliation to the account's history and stamps the
// balance as of the statement. It then retires the provisional balance checkpoints the statement
// supersedes, because the statement's own rows now carry that balance. A normal and a forced
// reconciliation both go through here, so both retire the same checkpoints.
//
// It returns an error only when the reconciliation itself could not be saved. A checkpoint that
// cannot be retired is reported in the outcome.
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
	account.Reconciliations = reconcile.Record(account.Reconciliations, ev)
	account.BalanceAsOf = a.clock()
	if err := a.PutAccount(account); err != nil {
		return RecordOutcome{}, fmt.Errorf("record reconciliation: save account: %w", err)
	}
	var out RecordOutcome
	var failures []error
	a.BulkMutate(func() {
		for _, cid := range provisional.SupersededBy(accountID, a.Transactions(), ev.Through()) {
			if err := a.DeleteTransaction(cid); err != nil {
				failures = append(failures, fmt.Errorf("retire checkpoint %s: %w", cid, err))
				continue
			}
			out.Retired++
		}
	})
	out.RetireFailures = errors.Join(failures...)
	return out, nil
}
