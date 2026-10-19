// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PutTransactionsAtomic stores all transactions together or stores none of them.
func (s *SQLiteStore) PutTransactionsAtomic(txns []domain.Transaction) error {
	if len(txns) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin transaction batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare("INSERT INTO transactions(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data")
	if err != nil {
		return fmt.Errorf("store: prepare transaction batch: %w", err)
	}
	defer stmt.Close()
	for _, txn := range txns {
		data, err := json.Marshal(txn)
		if err != nil {
			return fmt.Errorf("store: encode transaction %s: %w", txn.ID, err)
		}
		if _, err := stmt.Exec(txn.ID, string(data)); err != nil {
			return fmt.Errorf("store: write transaction %s: %w", txn.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit transaction batch: %w", err)
	}
	mutationRev.Add(1)
	return nil
}

// DeleteTransactionsAtomic deletes existing transaction IDs in one transaction
// and returns only the IDs that existed and were deleted after commit.
func (s *SQLiteStore) DeleteTransactionsAtomic(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("store: begin transaction delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare("DELETE FROM transactions WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("store: prepare transaction delete: %w", err)
	}
	defer stmt.Close()
	deleted := make([]string, 0, len(ids))
	for _, id := range ids {
		result, err := stmt.Exec(id)
		if err != nil {
			return nil, fmt.Errorf("store: delete transaction %s: %w", id, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("store: count deleted transaction %s: %w", id, err)
		}
		if n > 0 {
			deleted = append(deleted, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit transaction delete: %w", err)
	}
	if len(deleted) > 0 {
		mutationRev.Add(1)
	}
	return deleted, nil
}
