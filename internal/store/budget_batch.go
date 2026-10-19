// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PutBudgetsAtomic writes all budgets in one transaction. Each row uses the
// same JSON upsert shape as PutBudget, and the mutation revision advances only
// after the transaction commits.
func (s *SQLiteStore) PutBudgetsAtomic(budgets []domain.Budget) error {
	if len(budgets) == 0 {
		return nil
	}
	prepared := make([]domain.Budget, len(budgets))
	copy(prepared, budgets)
	needsBaseCurrency := false
	seen := make(map[string]bool, len(prepared))
	for _, budget := range prepared {
		if budget.ID == "" {
			return fmt.Errorf("store: budget id is required")
		}
		if seen[budget.ID] {
			return fmt.Errorf("store: budget %q appears more than once in batch", budget.ID)
		}
		seen[budget.ID] = true
		if budget.Limit.Currency == "" || (budget.HasTarget() && budget.TargetAmount.Currency == "") {
			needsBaseCurrency = true
		}
	}
	if needsBaseCurrency {
		settings, err := s.GetSettings()
		if err != nil {
			return fmt.Errorf("store: resolve budget batch currency: %w", err)
		}
		for i := range prepared {
			prepared[i] = prepared[i].WithBaseCurrency(settings.BaseCurrency)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin budget batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare("INSERT INTO budgets(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data")
	if err != nil {
		return fmt.Errorf("store: prepare budget batch: %w", err)
	}
	defer stmt.Close()
	for _, budget := range prepared {
		if err := putBudgetJSONTx(stmt, budget); err != nil {
			return fmt.Errorf("store: write budget %s: %w", budget.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit budget batch: %w", err)
	}
	for range prepared {
		mutationRev.Add(1)
	}
	return nil
}

func putBudgetJSONTx(stmt *sql.Stmt, budget domain.Budget) error {
	data, err := json.Marshal(budget)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if _, err := stmt.Exec(budget.ID, string(data)); err != nil {
		return err
	}
	return nil
}
