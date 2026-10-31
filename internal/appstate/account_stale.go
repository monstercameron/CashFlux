// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// MarkBalancesUpdated stamps each named account's balance as confirmed at now and
// returns how many distinct accounts were written. A repeated id counts once.
//
// Every account is looked up before the first write, so an unknown id changes nothing.
// An account that no longer validates is skipped and reported in the returned error,
// one entry per account; the others are stamped in a single commit, so a storage
// failure stamps none of them and returns a count of zero.
func (a *App) MarkBalancesUpdated(ids []string, now time.Time) (int, error) {
	if err := a.roleGuard(); err != nil {
		return 0, err
	}
	if now.IsZero() {
		now = a.clock()
	}
	seen := make(map[string]bool, len(ids))
	changes := make([]store.RecordChange, 0, len(ids))
	var failures []error
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		account, found, err := a.store.GetAccount(id)
		if err != nil {
			return 0, fmt.Errorf("mark balances updated: load account %q: %w", id, err)
		}
		if !found {
			return 0, fmt.Errorf("mark balances updated: %w: %q", ErrAccountNotFound, id)
		}
		before, err := json.Marshal(account)
		if err != nil {
			return 0, fmt.Errorf("mark balances updated: encode account %q: %w", id, err)
		}
		account.BalanceAsOf = now
		if is := validate.ValidateAccount(account); !is.OK() {
			failures = append(failures, fmt.Errorf("mark balance updated for account %q: %w", id, is))
			continue
		}
		after, err := json.Marshal(account)
		if err != nil {
			return 0, fmt.Errorf("mark balances updated: encode account %q: %w", id, err)
		}
		changes = append(changes, store.RecordChange{Table: "accounts", ID: id, Before: before, After: after})
	}
	if err := a.store.ReplayChanges(changes); err != nil {
		return 0, fmt.Errorf("mark balances updated: %w", err)
	}
	return len(changes), errors.Join(failures...)
}
