// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// visitBuckets assigns each transaction to its half-open reporting bucket.
// Ordered bounds need one ledger pass instead of one pass per bucket. Keep the
// historical behavior for duplicate or descending bounds: each adjacent pair
// remains an independent range, including overlapping ranges.
func visitBuckets(txns []domain.Transaction, bounds []time.Time, visit func(int, domain.Transaction) error) error {
	if len(bounds) < 2 {
		return nil
	}
	ordered := true
	for i := 1; i < len(bounds); i++ {
		if !bounds[i].After(bounds[i-1]) {
			ordered = false
			break
		}
	}
	if !ordered {
		for i := 0; i+1 < len(bounds); i++ {
			for _, txn := range txns {
				if dateutil.InRange(txn.Date, bounds[i], bounds[i+1]) {
					if err := visit(i, txn); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, txn := range txns {
		if txn.Date.Before(bounds[0]) || !txn.Date.Before(bounds[len(bounds)-1]) {
			continue
		}
		i := sort.Search(len(bounds)-1, func(i int) bool { return txn.Date.Before(bounds[i+1]) })
		if err := visit(i, txn); err != nil {
			return err
		}
	}
	return nil
}
