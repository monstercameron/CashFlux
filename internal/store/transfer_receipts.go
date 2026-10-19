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

func replaceTransferReceipts(tx *sql.Tx, receipts []TransferOperationReceipt) error {
	if _, err := tx.Exec("DELETE FROM transfer_operations"); err != nil {
		return err
	}
	for _, receipt := range receipts {
		if receipt.ID == "" {
			return fmt.Errorf("store: transfer receipt id is required")
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

func (s *SQLiteStore) transferReceipts() ([]TransferOperationReceipt, error) {
	rows, err := s.db.Query("SELECT id,data FROM transfer_operations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
	return receipts, rows.Err()
}
