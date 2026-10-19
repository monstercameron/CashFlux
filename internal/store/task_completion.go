// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PutTasksAtomic stores all supplied tasks in one transaction or stores none.
func (s *SQLiteStore) PutTasksAtomic(tasks ...domain.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin task completion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO tasks(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data")
	if err != nil {
		return fmt.Errorf("store: prepare task completion: %w", err)
	}
	defer stmt.Close()
	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.ID == "" {
			return fmt.Errorf("store: task completion: id is required")
		}
		if _, duplicate := seen[task.ID]; duplicate {
			return fmt.Errorf("store: task completion: duplicate task id %q", task.ID)
		}
		seen[task.ID] = struct{}{}
		data, err := json.Marshal(task)
		if err != nil {
			return fmt.Errorf("store: encode task %q for completion: %w", task.ID, err)
		}
		if _, err := stmt.Exec(task.ID, string(data)); err != nil {
			return fmt.Errorf("store: write task %q for completion: %w", task.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit task completion: %w", err)
	}
	mutationRev.Add(1)
	return nil
}
