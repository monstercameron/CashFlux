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
	sort.Slice(cp, func(i, j int) bool { return cp[i].Date.After(cp[j].Date) })
	if n < 0 {
		n = 0
	}
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
