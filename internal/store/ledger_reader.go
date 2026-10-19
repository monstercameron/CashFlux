// SPDX-License-Identifier: MIT

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// LedgerReader reads transaction-only changes from the store capture. Its
// watermark survives pruning and participates in the writer's SQL transaction.
// Readers belong to one store, never to a workspace-independent global cache.
type LedgerReader struct{ store *SQLiteStore }

// NewLedgerReader installs a store-local transaction watermark. TEMP read-side
// infrastructure leaves portable datasets and the main schema unchanged.
func (s *SQLiteStore) NewLedgerReader() (*LedgerReader, error) {
	_, err := s.db.Exec(`CREATE TEMP TABLE IF NOT EXISTS ledger_read_head (id INTEGER PRIMARY KEY CHECK(id=1), seq INTEGER NOT NULL);
INSERT OR IGNORE INTO ledger_read_head VALUES(1, COALESCE((SELECT seq FROM temp.sqlite_sequence WHERE name='store_change_journal'),0));
CREATE TEMP TRIGGER IF NOT EXISTS ledger_read_capture AFTER INSERT ON store_change_journal
WHEN NEW.table_name='transactions' BEGIN UPDATE ledger_read_head SET seq=NEW.seq WHERE id=1; END;`)
	if err != nil {
		return nil, fmt.Errorf("store: initialize ledger reader: %w", err)
	}
	return &LedgerReader{store: s}, nil
}

// Revision returns the last transaction-table mutation, including deletions,
// replacements and undo replay. Capture pruning cannot move it backwards.
func (r *LedgerReader) Revision() (uint64, error) {
	var revision uint64
	if err := r.store.db.QueryRow("SELECT seq FROM ledger_read_head WHERE id=1").Scan(&revision); err != nil {
		return 0, fmt.Errorf("store: read ledger revision: %w", err)
	}
	return revision, nil
}

// Changes returns transaction transitions in a fixed revision interval.
// ErrCaptureExpired requires a complete ledger reload, never a partial update.
func (r *LedgerReader) Changes(after, through uint64) ([]RecordChange, error) {
	tx, err := r.store.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("store: begin ledger read: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var pruned, head uint64
	if err := tx.QueryRow("SELECT pruned,(SELECT seq FROM ledger_read_head WHERE id=1) FROM store_capture_meta").Scan(&pruned, &head); err != nil {
		return nil, fmt.Errorf("store: read ledger capture watermark: %w", err)
	}
	if through < after || through > head {
		return nil, fmt.Errorf("store: invalid ledger change range")
	}
	if after < pruned {
		return nil, ErrCaptureExpired
	}
	rows, err := tx.Query(`SELECT record_id,before_value,after_value FROM store_change_journal
WHERE seq>? AND seq<=? AND table_name='transactions' ORDER BY seq`, after, through)
	if err != nil {
		return nil, fmt.Errorf("store: read ledger changes: %w", err)
	}
	defer rows.Close()
	var changes []RecordChange
	for rows.Next() {
		var change RecordChange
		var before, after sql.NullString
		if err := rows.Scan(&change.ID, &before, &after); err != nil {
			return nil, fmt.Errorf("store: decode ledger change: %w", err)
		}
		change.Table = "transactions"
		if before.Valid {
			change.Before = json.RawMessage(before.String)
		}
		if after.Valid {
			change.After = json.RawMessage(after.String)
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate ledger changes: %w", err)
	}
	return CoalesceChanges(changes), nil
}
