// SPDX-License-Identifier: MIT

package appstate

import "github.com/monstercameron/CashFlux/internal/domain"

// ledgerSnapshot is the immutable, revision-scoped view of the transaction table.
// Its slices contain domain values whose nested slices must also be treated as read-only.
type ledgerSnapshot struct {
	rows      []domain.Transaction
	byID      map[string]int
	byAccount map[string][]domain.Transaction
}

// ledgerSnapshotCache builds the ledger rows and indexes once for each store revision.
// App state is accessed on the single-threaded wasm runtime; native tests use it serially.
type ledgerSnapshotCache struct {
	rev    uint64
	value  *ledgerSnapshot
	loaded bool
}

func (c *ledgerSnapshotCache) get(rev uint64, load func() []domain.Transaction) *ledgerSnapshot {
	if !c.loaded || c.rev != rev {
		rows := load()
		snapshot := &ledgerSnapshot{
			rows:      rows,
			byID:      make(map[string]int, len(rows)),
			byAccount: make(map[string][]domain.Transaction),
		}
		accountCounts := make(map[string]int)
		for _, row := range rows {
			accountCounts[row.AccountID]++
		}
		for accountID, count := range accountCounts {
			snapshot.byAccount[accountID] = make([]domain.Transaction, 0, count)
		}
		for i, row := range rows {
			snapshot.byID[row.ID] = i
			snapshot.byAccount[row.AccountID] = append(snapshot.byAccount[row.AccountID], row)
		}
		c.value, c.rev, c.loaded = snapshot, rev, true
	}
	return c.value
}
