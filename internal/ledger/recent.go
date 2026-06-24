// SPDX-License-Identifier: MIT

package ledger

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Recent returns the n most recent transactions, newest first, without mutating
// the input slice. n <= 0 yields an empty result; n beyond the input length
// returns all of them.
func Recent(txns []domain.Transaction, n int) []domain.Transaction {
	cp := append([]domain.Transaction(nil), txns...)
	// Newest first, breaking ties on ID so equal-dated transactions order
	// deterministically rather than depending on the input's arrangement.
	sort.Slice(cp, func(i, j int) bool {
		if !cp[i].Date.Equal(cp[j].Date) {
			return cp[i].Date.After(cp[j].Date)
		}
		return cp[i].ID < cp[j].ID
	})
	if n < 0 {
		n = 0
	}
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
