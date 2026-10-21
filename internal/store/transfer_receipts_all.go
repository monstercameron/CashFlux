// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"fmt"
)

// ListAllTransferReceipts reads every actually retained operation receipt.
// Unlike the checkpoint loader, it neither installs cleanup triggers nor prunes
// legacy receipts whose transaction legs are no longer present.
func (s *SQLiteStore) ListAllTransferReceipts() ([]TransferOperationReceipt, error) {
	rows, err := s.db.Query("SELECT id,data FROM transfer_operations ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("store: read retained transfer receipts: %w", err)
	}
	defer rows.Close()
	out := []TransferOperationReceipt{}
	for rows.Next() {
		var receipt TransferOperationReceipt
		var raw string
		if err := rows.Scan(&receipt.ID, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &receipt.Transactions); err != nil {
			return nil, fmt.Errorf("store: transfer receipt %q unavailable: %w", receipt.ID, err)
		}
		out = append(out, receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}
