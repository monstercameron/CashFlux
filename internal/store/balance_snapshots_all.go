// SPDX-License-Identifier: MIT

package store

import "github.com/monstercameron/CashFlux/internal/domain"

// ListAllBalanceSnapshots returns every retained historical balance row, including
// history whose account was removed. It does not load unrelated dataset tables.
func (s *SQLiteStore) ListAllBalanceSnapshots() ([]domain.BalanceSnapshot, error) {
	return loadRows[domain.BalanceSnapshot](s.db, "balance_snapshots")
}
