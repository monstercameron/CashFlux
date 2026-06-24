// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// preservedOnWipe is the set of tables a Wipe must NOT clear. Settings (base
// currency, FX table, theme/prefs) are configuration, not financial data, so they
// survive "wipe all data" — everything else (financial data and anything derived
// from it: transactions, budgets, goals, recurring, plans, subscriptions, earmarks,
// insights, conversations, workflows, audit, …) is removed.
var preservedOnWipe = map[string]bool{"settings": true, "settingskv": true}

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

// Wipe removes all financial data (and everything derived from it) from the store
// while preserving settings (see preservedOnWipe). It enumerates the live tables
// from the schema so every entity table — including any added later — is cleared
// automatically; a partial list is exactly how stale recurring/plans/subscriptions
// data used to survive a wipe. It is atomic.
func (s *SQLiteStore) Wipe() error {
	// Discover every user table from the schema (excluding SQLite internals).
	rows, err := s.db.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return err
		}
		if !preservedOnWipe[name] {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, t := range tables {
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
