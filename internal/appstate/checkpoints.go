// SPDX-License-Identifier: MIT

package appstate

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/checkpoints"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// AccountBalanceSeries returns the anchored daily balance series for an account
// across the inclusive day range [start, end] (AC6). It prefers the account's
// recorded balance snapshots — the trusted anchors written by the update-balance /
// reconcile flow — and applies transactions since the nearest anchor, so sparse
// ledgers do not drift. This is the read-model AC2's per-account sparkline consumes
// so a flat stretch since the last confirmed balance reads as honest, not missing
// data. Returns nil for an unknown account or an inverted range.
func (a *App) AccountBalanceSeries(accountID string, start, end time.Time) []checkpoints.Point {
	acc, ok := a.findAccount(accountID)
	if !ok {
		return nil
	}
	anchors := checkpoints.ForAccount(a.BalanceHistory(accountID), accountID)
	return checkpoints.Series(acc, a.Transactions(), anchors, start, end)
}

// AccountBalanceAsOf returns an account's anchored balance on a given day (AC6),
// preferring the nearest confirmed snapshot then applying transactions since it.
func (a *App) AccountBalanceAsOf(accountID string, day time.Time) (int64, bool) {
	acc, ok := a.findAccount(accountID)
	if !ok {
		return 0, false
	}
	anchors := checkpoints.ForAccount(a.BalanceHistory(accountID), accountID)
	return checkpoints.BalanceMinorAt(acc, a.Transactions(), anchors, day), true
}

// findAccount looks up an account by ID from the current set.
func (a *App) findAccount(id string) (domain.Account, bool) {
	for _, ac := range a.Accounts() {
		if ac.ID == id {
			return ac, true
		}
	}
	return domain.Account{}, false
}
