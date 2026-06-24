// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"errors"
)

// The appkv table holds app/UI key-value state (dashboard layout, widget config,
// view filters, the activity feed, …) that used to live in scattered localStorage
// keys. Centralizing it in SQLite makes the dataset the single source of truth and
// ensures a Wipe (which clears every non-settings table) takes it with everything
// else. Each value is an opaque string the wasm layer encodes/decodes.

// loadKV reads the whole appkv table into a map (nil when empty).
func loadKV(db *sql.DB) (map[string]string, error) { return loadKVTable(db, "appkv") }

// loadSettingsKV reads the whole settingskv table into a map (nil when empty).
func loadSettingsKV(db *sql.DB) (map[string]string, error) { return loadKVTable(db, "settingskv") }

func loadKVTable(db *sql.DB, table string) (map[string]string, error) {
	rows, err := db.Query("SELECT k, v FROM " + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out map[string]string
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// replaceKVTable clears a kv table and reinserts the map (used by Load).
func replaceKVTable(tx *sql.Tx, table string, kv map[string]string) error {
	if _, err := tx.Exec("DELETE FROM " + table); err != nil {
		return err
	}
	if len(kv) == 0 {
		return nil
	}
	stmt, err := tx.Prepare("INSERT INTO " + table + "(k, v) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for k, v := range kv {
		if _, err := stmt.Exec(k, v); err != nil {
			return err
		}
	}
	return nil
}

// GetKV returns the value for key and whether it was present.
func (s *SQLiteStore) GetKV(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow("SELECT v FROM appkv WHERE k = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetKV upserts a key-value pair and advances the mutation revision so memoized
// derived values recompute (§1.6) and the autosave persists the change.
func (s *SQLiteStore) SetKV(key, val string) error {
	_, err := s.db.Exec(
		"INSERT INTO appkv(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v",
		key, val,
	)
	if err != nil {
		return err
	}
	mutationRev.Add(1)
	return nil
}

// DeleteKV removes a key (no-op if absent) and advances the mutation revision.
func (s *SQLiteStore) DeleteKV(key string) error {
	if _, err := s.db.Exec("DELETE FROM appkv WHERE k = ?", key); err != nil {
		return err
	}
	mutationRev.Add(1)
	return nil
}

// GetSettingKV / SetSettingKV / DeleteSettingKV are the preserved-on-wipe
// counterparts for config and preferences (theme, fonts, language, prefs, …).

func (s *SQLiteStore) GetSettingKV(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow("SELECT v FROM settingskv WHERE k = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *SQLiteStore) SetSettingKV(key, val string) error {
	_, err := s.db.Exec(
		"INSERT INTO settingskv(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v",
		key, val,
	)
	if err != nil {
		return err
	}
	mutationRev.Add(1)
	return nil
}

func (s *SQLiteStore) DeleteSettingKV(key string) error {
	if _, err := s.db.Exec("DELETE FROM settingskv WHERE k = ?", key); err != nil {
		return err
	}
	mutationRev.Add(1)
	return nil
}
