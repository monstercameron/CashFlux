// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PutRecurringOccurrenceAtomic records an occurrence receipt, its transaction
// rows, and the schedule advance together. The exact schedule before-image
// guards against a concurrent edit; an existing receipt never posts again,
// including after its transaction was edited or deleted.
func (s *SQLiteStore) PutRecurringOccurrenceAtomic(operationID string, before, after domain.Recurring, txns []domain.Transaction) (bool, error) {
	if operationID == "" || before.ID == "" || before.ID != after.ID || len(txns) == 0 || !after.NextDue.After(before.NextDue) {
		return false, fmt.Errorf("store: invalid recurring occurrence")
	}
	prior, err := json.Marshal(before)
	if err != nil {
		return false, fmt.Errorf("store: encode recurring before-image: %w", err)
	}
	next, err := json.Marshal(after)
	if err != nil {
		return false, fmt.Errorf("store: encode recurring advance: %w", err)
	}
	receipt, err := json.Marshal(txns)
	if err != nil {
		return false, fmt.Errorf("store: encode recurring receipt: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("store: begin recurring occurrence: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var current string
	if err := tx.QueryRow("SELECT data FROM recurring WHERE id=?", before.ID).Scan(&current); err != nil {
		return false, fmt.Errorf("store: read recurring occurrence: %w", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(current)); err != nil {
		return false, fmt.Errorf("store: decode recurring before-image: %w", err)
	}
	var existing string
	err = tx.QueryRow("SELECT v FROM appkv WHERE k=?", "recurring-occurrence:"+operationID).Scan(&existing)
	posted := err == sql.ErrNoRows
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("store: read recurring receipt: %w", err)
	}
	if !bytes.Equal(compact.Bytes(), prior) {
		if !posted {
			return false, nil
		} // already posted; never rewind a newer schedule
		return false, fmt.Errorf("store: recurring %s changed before posting", before.ID)
	}
	if posted {
		for _, txn := range txns {
			if txn.ID == "" {
				return false, fmt.Errorf("store: recurring transaction id is required")
			}
			data, err := json.Marshal(txn)
			if err != nil {
				return false, fmt.Errorf("store: encode recurring transaction: %w", err)
			}
			if _, err := tx.Exec("INSERT INTO transactions(id,data) VALUES(?,?)", txn.ID, string(data)); err != nil {
				return false, fmt.Errorf("store: insert recurring transaction: %w", err)
			}
		}
		if _, err := tx.Exec("INSERT INTO appkv(k,v) VALUES(?,?)", "recurring-occurrence:"+operationID, string(receipt)); err != nil {
			return false, fmt.Errorf("store: record recurring occurrence: %w", err)
		}
	}
	if _, err := tx.Exec("UPDATE recurring SET data=? WHERE id=?", string(next), before.ID); err != nil {
		return false, fmt.Errorf("store: advance recurring: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit recurring occurrence: %w", err)
	}
	mutationRev.Add(1)
	return posted, nil
}

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

// PutTransferBatchAtomic commits the operation receipt and every transaction
// together. Repeating the same operation ID and payload is a no-op; reusing an
// operation ID for different data is rejected without overwriting anything.
func (s *SQLiteStore) PutTransferBatchAtomic(operationID string, txns []domain.Transaction) (bool, error) {
	if operationID == "" {
		return false, fmt.Errorf("store: transfer operation id is required")
	}
	if len(txns) == 0 {
		return false, fmt.Errorf("store: transfer batch must contain transactions")
	}
	data, err := json.Marshal(txns)
	if err != nil {
		return false, fmt.Errorf("store: encode transfer operation %s: %w", operationID, err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("store: begin transfer batch %s: %w", operationID, err)
	}
	defer func() { _ = tx.Rollback() }()
	var existingBatch string
	receiptErr := tx.QueryRow("SELECT data FROM transfer_operations WHERE id = ?", operationID).Scan(&existingBatch)
	if receiptErr != nil && receiptErr != sql.ErrNoRows {
		return false, fmt.Errorf("store: read transfer operation %s: %w", operationID, receiptErr)
	}
	if receiptErr == nil {
		if existingBatch != string(data) {
			return false, fmt.Errorf("store: transfer operation %s already belongs to different data", operationID)
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("store: finish transfer retry %s: %w", operationID, err)
		}
		return false, nil
	}
	existingCount := 0
	encoded := make([]string, len(txns))
	for i, txn := range txns {
		if txn.ID == "" {
			return false, fmt.Errorf("store: transaction id is required")
		}
		txnData, err := json.Marshal(txn)
		if err != nil {
			return false, fmt.Errorf("store: encode transaction %s: %w", txn.ID, err)
		}
		encoded[i] = string(txnData)
		var existing string
		lookupErr := tx.QueryRow("SELECT data FROM transactions WHERE id = ?", txn.ID).Scan(&existing)
		if lookupErr == nil {
			if existing != string(txnData) {
				return false, fmt.Errorf("store: transaction id %s already belongs to different data", txn.ID)
			}
			existingCount++
		} else if lookupErr != sql.ErrNoRows {
			return false, fmt.Errorf("store: inspect transaction %s: %w", txn.ID, lookupErr)
		}
	}
	if existingCount != 0 && existingCount != len(txns) {
		return false, fmt.Errorf("store: transfer operation %s has an incomplete existing batch", operationID)
	}
	if _, err := tx.Exec("INSERT INTO transfer_operations(id, data) VALUES(?, ?)", operationID, string(data)); err != nil {
		return false, fmt.Errorf("store: reserve transfer operation %s: %w", operationID, err)
	}
	if existingCount == 0 {
		for i, txn := range txns {
			if _, err := tx.Exec("INSERT INTO transactions(id, data) VALUES(?, ?)", txn.ID, encoded[i]); err != nil {
				return false, fmt.Errorf("store: write transaction %s: %w", txn.ID, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit transfer batch %s: %w", operationID, err)
	}
	mutationRev.Add(1) // the new operation receipt is data even when its rows already existed
	if existingCount == 0 {
		return true, nil
	}
	return false, nil
}

// PutImportBatch stores imported transactions and, when doc has an ID, their
// history record in one SQLite transaction. Existing IDs are rejected instead
// of being overwritten.
func (s *SQLiteStore) PutImportBatch(txns []domain.Transaction, doc domain.Document) error {
	return s.PutImportBatchWithChanges(txns, doc, nil)
}

// PutImportBatchWithChanges stores new imported rows, their history, and planned
// automation effects in one transaction. Auxiliary transitions must match their
// exact before-images; an intervening edit aborts the entire import.
func (s *SQLiteStore) PutImportBatchWithChanges(txns []domain.Transaction, doc domain.Document, changes []RecordChange) error {
	if len(txns) == 0 && doc.ID == "" && len(changes) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin import batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare("INSERT INTO transactions(id, data) VALUES(?, ?)")
	if err != nil {
		return fmt.Errorf("store: prepare import batch: %w", err)
	}
	defer stmt.Close()
	for _, txn := range txns {
		if txn.ID == "" {
			return fmt.Errorf("store: import transaction id is required")
		}
		data, err := json.Marshal(txn)
		if err != nil {
			return fmt.Errorf("store: encode import transaction %s: %w", txn.ID, err)
		}
		if _, err := stmt.Exec(txn.ID, string(data)); err != nil {
			return fmt.Errorf("store: write import transaction %s: %w", txn.ID, err)
		}
	}
	if err := writeImportChanges(tx, changes); err != nil {
		return err
	}
	if doc.ID != "" {
		docData, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("store: encode import history %s: %w", doc.ID, err)
		}
		if _, err := tx.Exec("INSERT INTO documents(id, data) VALUES(?, ?)", doc.ID, string(docData)); err != nil {
			return fmt.Errorf("store: write import history %s: %w", doc.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit import batch: %w", err)
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
