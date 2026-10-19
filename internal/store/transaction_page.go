// SPDX-License-Identifier: MIT

package store

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TransactionsPage reads a bounded keyset page for cooperative background jobs.
// Callers must pin/validate the store revision between pages; a changed revision
// requires restarting the job. The ledger is never truncated by this API.
func (s *SQLiteStore) TransactionsPage(afterID string, limit int) ([]domain.Transaction, error) {
	if limit <= 0 || limit > 256 {
		return nil, fmt.Errorf("store: transaction page limit must be between 1 and 256")
	}
	return queryRows[domain.Transaction](s.db, "SELECT data FROM transactions WHERE id > ? ORDER BY id LIMIT ?", afterID, limit)
}
