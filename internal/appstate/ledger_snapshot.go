// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

// ledgerSnapshot is an immutable transaction-table view, including nested values.
type ledgerSnapshot struct {
	rows      []domain.Transaction
	byID      map[string]int
	byAccount map[string][]domain.Transaction
}

// ledgerSnapshotCache is store-local. Global revisions avoid even a watermark
// query on repeat reads; only transaction changes replace the immutable value.
type ledgerSnapshotCache struct {
	store       *store.SQLiteStore
	reader      *store.LedgerReader
	rev, cursor uint64
	value       *ledgerSnapshot
}

func (a *App) ledgerSnapshot() *ledgerSnapshot {
	v, err := a.reads.transactions.get(a.store)
	a.logErr("transaction snapshot", err)
	if v == nil {
		return &ledgerSnapshot{}
	}
	return v
}

func (c *ledgerSnapshotCache) get(s *store.SQLiteStore) (*ledgerSnapshot, error) {
	if c.store != s {
		*c = ledgerSnapshotCache{store: s}
	}
	if c.value != nil && c.rev == s.Rev() {
		return c.value, nil
	}
	if c.reader == nil {
		reader, err := s.NewLedgerReader()
		if err != nil {
			return nil, err
		}
		c.reader = reader
	}
	cursor, err := c.reader.Revision()
	if err != nil {
		return nil, err
	}
	if c.value != nil && c.cursor == cursor {
		c.rev = s.Rev()
		return c.value, nil
	}
	var next *ledgerSnapshot
	if c.value != nil {
		changes, captureErr := c.reader.Changes(c.cursor, cursor)
		if captureErr == nil {
			next, err = c.value.apply(changes)
			if err != nil {
				return nil, err
			}
		}
		if captureErr != nil && !errors.Is(captureErr, store.ErrCaptureExpired) {
			return nil, captureErr
		}
		// Pruned history is never guessed: reload committed SQL.
	}
	if next == nil {
		rows, err := s.ListTransactions()
		if err != nil {
			return nil, fmt.Errorf("load ledger snapshot: %w", err)
		}
		next = newLedgerSnapshot(rows)
	}
	c.value, c.cursor, c.rev = next, cursor, s.Rev()
	return next, nil
}

func newLedgerSnapshot(rows []domain.Transaction) *ledgerSnapshot {
	s := &ledgerSnapshot{rows: rows, byID: make(map[string]int, len(rows)), byAccount: map[string][]domain.Transaction{}}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.AccountID]++
	}
	for account, count := range counts {
		s.byAccount[account] = make([]domain.Transaction, 0, count)
	}
	for i, row := range rows {
		s.byID[row.ID] = i
		s.byAccount[row.AccountID] = append(s.byAccount[row.AccountID], row)
	}
	return s
}

// apply copies the row slice once and shares unchanged ID/account indexes.
// Ordinary edits decode only after-images and copy only affected accounts.
// Structural changes retain ID order, matching ListTransactions exactly.
func (s *ledgerSnapshot) apply(changes []store.RecordChange) (*ledgerSnapshot, error) {
	if len(changes) == 0 {
		return s, nil
	}
	next := &ledgerSnapshot{rows: slices.Clone(s.rows), byID: s.byID, byAccount: maps.Clone(s.byAccount)}
	affected := map[string]bool{}
	updates := make(map[string]domain.Transaction, len(changes))
	deleted := map[string]bool{}
	structural := false
	for _, change := range changes {
		index, exists := s.byID[change.ID]
		if exists {
			affected[s.rows[index].AccountID] = true
		}
		if change.After == nil {
			deleted[change.ID] = true
			structural = structural || exists
			continue
		}
		var row domain.Transaction
		if err := json.Unmarshal(change.After, &row); err != nil {
			return nil, fmt.Errorf("decode ledger row %q: %w", change.ID, err)
		}
		affected[row.AccountID] = true
		updates[change.ID] = row
		if exists {
			next.rows[index] = row
		} else {
			structural = true
		}
	}
	if structural {
		next.rows = next.rows[:0]
		for _, row := range s.rows {
			if deleted[row.ID] {
				continue
			}
			if update, ok := updates[row.ID]; ok {
				row = update
			}
			next.rows = append(next.rows, row)
		}
		for id, row := range updates {
			if _, exists := s.byID[id]; !exists {
				next.rows = append(next.rows, row)
			}
		}
		sort.Slice(next.rows, func(i, j int) bool { return next.rows[i].ID < next.rows[j].ID })
		next.byID = make(map[string]int, len(next.rows))
		for i, row := range next.rows {
			next.byID[row.ID] = i
		}
	}
	if len(next.rows) == 0 {
		next.rows = nil
	}
	for account := range affected {
		// A single allocation per affected account avoids repeated copying of large
		// Transaction values as append grows the account slice.
		next.byAccount[account] = make([]domain.Transaction, 0, len(s.byAccount[account])+len(updates))
	}
	for _, row := range next.rows {
		if affected[row.AccountID] {
			next.byAccount[row.AccountID] = append(next.byAccount[row.AccountID], row)
		}
	}
	for account := range affected {
		if len(next.byAccount[account]) == 0 {
			delete(next.byAccount, account)
		}
	}
	return next, nil
}
