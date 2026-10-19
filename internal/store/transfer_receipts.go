// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TransferOperationReceipt preserves operation-id deduplication across full
// checkpoints, imports and sync. It is part of the dataset, not a device secret.
type TransferOperationReceipt struct {
	ID           string               `json:"id"`
	Transactions []domain.Transaction `json:"transactions"`
}

func liveTransferReceipts(receipts []TransferOperationReceipt, transactions []domain.Transaction) []TransferOperationReceipt {
	transactionIDs := make(map[string]struct{}, len(transactions))
	for _, txn := range transactions {
		transactionIDs[txn.ID] = struct{}{}
	}
	filtered := make([]TransferOperationReceipt, 0, len(receipts))
	for _, receipt := range receipts {
		if receipt.ID == "" || len(receipt.Transactions) == 0 {
			continue
		}
		live := true
		for _, txn := range receipt.Transactions {
			if txn.ID == "" {
				live = false
				break
			}
			if _, ok := transactionIDs[txn.ID]; !ok {
				live = false
				break
			}
		}
		if live {
			filtered = append(filtered, receipt)
		}
	}
	return filtered
}

func replaceTransferReceipts(tx *sql.Tx, receipts []TransferOperationReceipt) error {
	// Keep legacy receipts from retaining transfers removed before this cleanup
	// hook existed, and remove receipts transactionally whenever a leg is deleted.
	if err := ensureTransferReceiptCleanup(tx); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM transfer_operations"); err != nil {
		return err
	}
	for _, receipt := range receipts {
		if receipt.ID == "" {
			return fmt.Errorf("store: transfer receipt id is required")
		}
		live, err := receiptHasAllTransactions(tx, receipt.Transactions)
		if err != nil {
			return fmt.Errorf("store: check transfer receipt %q: %w", receipt.ID, err)
		}
		if !live {
			continue
		}
		data, err := json.Marshal(receipt.Transactions)
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO transfer_operations(id,data) VALUES(?,?)", receipt.ID, string(data)); err != nil {
			return fmt.Errorf("store: load transfer receipt %q: %w", receipt.ID, err)
		}
	}
	return nil
}

type transferReceiptExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func ensureTransferReceiptCleanup(db transferReceiptExecer) error {
	_, err := db.Exec(`CREATE TRIGGER IF NOT EXISTS prune_transfer_receipts_after_transaction_delete
		AFTER DELETE ON transactions
		BEGIN
			DELETE FROM transfer_operations
			WHERE EXISTS (
				SELECT 1 FROM json_each(transfer_operations.data) AS leg
				WHERE json_extract(leg.value, '$.id') = OLD.id
			);
		END`)
	if err != nil {
		return fmt.Errorf("store: install transfer receipt cleanup: %w", err)
	}
	return nil
}

type transferReceiptQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func receiptHasAllTransactions(q transferReceiptQuerier, txns []domain.Transaction) (bool, error) {
	if len(txns) == 0 {
		return false, nil
	}
	for _, txn := range txns {
		if txn.ID == "" {
			return false, nil
		}
		var exists int
		err := q.QueryRow("SELECT 1 FROM transactions WHERE id = ?", txn.ID).Scan(&exists)
		if err == sql.ErrNoRows {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *SQLiteStore) transferReceipts() ([]TransferOperationReceipt, error) {
	if err := ensureTransferReceiptCleanup(s.db); err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT id,data FROM transfer_operations ORDER BY id")
	if err != nil {
		return nil, err
	}
	var receipts []TransferOperationReceipt
	for rows.Next() {
		var receipt TransferOperationReceipt
		var data string
		if err := rows.Scan(&receipt.ID, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &receipt.Transactions); err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	liveReceipts := receipts[:0]
	for _, receipt := range receipts {
		live, err := receiptHasAllTransactions(s.db, receipt.Transactions)
		if err != nil {
			return nil, fmt.Errorf("store: check transfer receipt %q: %w", receipt.ID, err)
		}
		if live {
			liveReceipts = append(liveReceipts, receipt)
			continue
		}
		if _, err := s.db.Exec("DELETE FROM transfer_operations WHERE id = ?", receipt.ID); err != nil {
			return nil, fmt.Errorf("store: prune transfer receipt %q: %w", receipt.ID, err)
		}
	}
	return liveReceipts, nil
}
