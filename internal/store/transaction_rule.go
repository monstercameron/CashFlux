// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// PutTransactionWithRule stores a transaction and a rule in one SQLite
// transaction: both rows commit or neither does. Both ids are required.
func (s *SQLiteStore) PutTransactionWithRule(t domain.Transaction, r rules.Rule) error {
	if t.ID == "" {
		return fmt.Errorf("store: transactions: id is required")
	}
	if r.ID == "" {
		return fmt.Errorf("store: rules: id is required")
	}
	t.NormalizeOwners()
	txnData, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("store: encode transaction %s: %w", t.ID, err)
	}
	ruleData, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("store: encode rule %s: %w", r.ID, err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin transaction with rule: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	const upsert = "INSERT INTO %s(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data"
	if _, err := tx.Exec(fmt.Sprintf(upsert, "transactions"), t.ID, string(txnData)); err != nil {
		return fmt.Errorf("store: write transaction %s: %w", t.ID, err)
	}
	if _, err := tx.Exec(fmt.Sprintf(upsert, "rules"), r.ID, string(ruleData)); err != nil {
		return fmt.Errorf("store: write rule %s: %w", r.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit transaction with rule: %w", err)
	}
	mutationRev.Add(1)
	return nil
}
