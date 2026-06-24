// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// allTables lists every entity table plus settings, in dependency-free order.
var allTables = []string{
	"members", "accounts", "categories", "transactions", "budgets", "goals", "tasks",
	"customfielddefs", "settings",
}

// GetSettings returns the stored settings, or the zero value if none are saved.
func (s *SQLiteStore) GetSettings() (Settings, error) {
	var data string
	err := s.db.QueryRow("SELECT data FROM settings WHERE id = 'app'").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var st Settings
	if err := json.Unmarshal([]byte(data), &st); err != nil {
		return Settings{}, err
	}
	return st, nil
}

// PutSettings saves (upserts) the settings.
func (s *SQLiteStore) PutSettings(st Settings) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		"INSERT INTO settings(id, data) VALUES('app', ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data",
		string(data),
	)
	if err != nil {
		return err
	}
	// Settings carry the base currency + FX table, which derived values (net worth,
	// totals) depend on, so a settings write must advance the mutation revision too
	// — otherwise a memo keyed on it would show a stale figure after an FX edit (§1.6).
	mutationRev.Add(1)
	return nil
}

// Wipe removes all data from the store (used by "wipe all data"). It is atomic.
func (s *SQLiteStore) Wipe() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, t := range allTables {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	mutationRev.Add(1) // a wipe changes everything; advance the memo key (§1.6)
	return nil
}
