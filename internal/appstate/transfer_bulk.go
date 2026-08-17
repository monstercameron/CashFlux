// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ClassifyTransfers writes a bulk transfer classification as ONE mutation, and
// leaves the ledger exactly as it found it if any row fails (C676).
//
// The rows come from txnclassify.PlanBulk, which has already refused the ones
// that cannot take the classification — so a failure here is a store-level
// refusal, not a foreseeable one, and it means the plan the user confirmed is no
// longer the plan being written. Writing the surviving half of it would leave a
// ledger nobody chose: some rows moved out of income, some still in it, and no
// record of which. So the priors are captured first and put back on the first
// error.
//
// The rollback goes through RestoreTransactions rather than a second manual loop
// because it is the same primitive bulk undo uses, and a restore that behaved
// differently from undo would be a second definition of "put it back".
//
// Returns how many rows were written. On error that count is zero, because the
// ledger is back at its starting point.
func (a *App) ClassifyTransfers(rows []domain.Transaction) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	if err := a.roleGuard(); err != nil {
		return 0, err
	}
	// The rows as they stand now, so a failure can put them back. Indexed from
	// the live ledger rather than trusting the caller's copies: the caller's are
	// the AFTER state, and a row that has changed under the surface since the plan
	// was computed must be restored to what the store holds, not to what the
	// screen remembers.
	//
	// A row that is no longer in the store at all has no prior, and PutTransaction
	// would CREATE it — so those ids are tracked separately and deleted on the way
	// back. Restoring only what was captured would leave a batch that "rolled
	// back" having resurrected a transaction the ledger had deliberately lost.
	live := make(map[string]domain.Transaction, len(rows))
	for _, t := range a.Transactions() {
		live[t.ID] = t
	}
	priors := make([]domain.Transaction, 0, len(rows))
	var created []string
	for _, r := range rows {
		if prior, ok := live[r.ID]; ok {
			priors = append(priors, prior)
			continue
		}
		created = append(created, r.ID)
	}

	var firstErr error
	written := 0
	// ONE observer fire for the whole operation, rollback included. Without the
	// outer BulkMutate the write's own fire lands between the failure and the
	// restore, showing every observer a ledger that is half-classified — a state
	// this function exists to make unobservable. Nested BulkMutate is safe: the
	// inner fire no-ops while suppression is still on.
	a.BulkMutate(func() {
		for _, t := range rows {
			if err := a.PutTransaction(t); err != nil {
				firstErr = fmt.Errorf("classify transaction %s: %w", t.ID, err)
				break
			}
			written++
		}
		if firstErr == nil {
			return
		}
		// Best-effort restore. If putting a row BACK also fails the dataset is in
		// a state this layer cannot reason about, so the original refusal is
		// reported alongside rather than replaced — the first error is the one
		// that explains what the user was trying to do.
		if rerr := a.RestoreTransactions(priors); rerr != nil {
			firstErr = fmt.Errorf("%w (and the ledger could not be restored: %v)", firstErr, rerr)
		}
		for _, id := range created {
			if derr := a.DeleteTransaction(id); derr != nil {
				firstErr = fmt.Errorf("%w (and a row created by the batch could not be removed: %v)", firstErr, derr)
			}
		}
	})
	if firstErr != nil {
		return 0, firstErr
	}
	return written, nil
}
