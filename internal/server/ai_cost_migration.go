// SPDX-License-Identifier: MIT

package server

import "fmt"

// migrateTo22 stores only daily estimated AI cost totals. Prompts, responses,
// keys, and per-call financial context are never added to the usage database.
func (s *Store) migrateTo22() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("server store: migrate v22 begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// A few older migration fixtures omit the v1 usage table entirely. Creating
	// its original shape here also makes the upgrade resilient to that legacy
	// partial schema without touching any existing usage rows.
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS usage (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  day TEXT NOT NULL,
  requests INTEGER NOT NULL DEFAULT 0,
  tokens INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(user_id, day)
)`); err != nil {
		return fmt.Errorf("server store: migrate v22 usage table: %w", err)
	}
	rows, err := tx.Query(`SELECT name FROM pragma_table_info('usage')`)
	if err != nil {
		return fmt.Errorf("server store: migrate v22 inspect usage: %w", err)
	}
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return fmt.Errorf("server store: migrate v22 scan usage: %w", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("server store: migrate v22 inspect usage: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("server store: migrate v22 close usage inspection: %w", err)
	}
	if !columns["estimated_cost_nano_usd"] {
		if _, err := tx.Exec(`ALTER TABLE usage ADD COLUMN estimated_cost_nano_usd INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("server store: migrate v22 add cost: %w", err)
		}
	}
	if !columns["cost_incomplete"] {
		if _, err := tx.Exec(`ALTER TABLE usage ADD COLUMN cost_incomplete INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("server store: migrate v22 add incomplete flag: %w", err)
		}
	}
	if !columns["estimated_cost_nano_usd"] {
		if _, err := tx.Exec(`UPDATE usage SET cost_incomplete = 1 WHERE tokens > 0`); err != nil {
			return fmt.Errorf("server store: migrate v22 mark earlier usage: %w", err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO schema_meta(id, version) VALUES(1, 22) ON CONFLICT(id) DO UPDATE SET version = excluded.version`); err != nil {
		return fmt.Errorf("server store: migrate v22 version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("server store: migrate v22 commit: %w", err)
	}
	return nil
}
