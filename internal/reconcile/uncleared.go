// SPDX-License-Identifier: MIT

package reconcile

import "github.com/monstercameron/CashFlux/internal/domain"

// Uncleared returns the transactions of one account that are not cleared yet, in their given order.
// The reconcile list, the "Mark all cleared" action and its preview all read this one selection.
func Uncleared(accountID string, txns []domain.Transaction) []domain.Transaction {
	var out []domain.Transaction
	for _, t := range txns {
		if t.AccountID == accountID && !t.Cleared {
			out = append(out, t)
		}
	}
	return out
}
