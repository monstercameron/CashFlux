// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ClassifyTransfers writes a bulk transfer classification as ONE atomic SQLite
// commit, leaving the ledger untouched if any row fails validation or storage
// (C676).
//
// Returns how many rows were written. On error that count is zero.
func (a *App) ClassifyTransfers(rows []domain.Transaction) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	if err := a.PutTransactionsAtomic(rows); err != nil {
		return 0, fmt.Errorf("classify transactions: %w", err)
	}
	return len(rows), nil
}
