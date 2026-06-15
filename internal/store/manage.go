package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// allTables lists every entity table plus settings, in dependency-free order.
var allTables = []string{
	"members", "accounts", "categories", "transactions", "budgets", "goals", "tasks", "settings",
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
	return err
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
	return tx.Commit()
}
