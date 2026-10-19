// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
)

const changeJournalTable = "store_change_journal"

// ErrCaptureExpired prevents a consumer from silently skipping pruned history.
var ErrCaptureExpired = errors.New("store: change capture cursor has expired")

// RecordChange is an exact transition of one stored record. Nil Before or After
// denotes absence; KV values are JSON strings. It never contains device keys.
type RecordChange struct {
	Table  string          `json:"table"`
	ID     string          `json:"id"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// CoalesceChanges retains the first before-image and final after-image of each
// record in a capture. Net no-ops disappear; first-touch order remains stable.
// It never mutates the input and retains the immutable JSON row bytes.
func CoalesceChanges(changes []RecordChange) []RecordChange {
	type key struct{ table, id string }
	indexes := map[key]int{}
	var records []RecordChange
	for _, change := range changes {
		k := key{change.Table, change.ID}
		if index, found := indexes[k]; found {
			records[index].After = change.After
			continue
		}
		indexes[k] = len(records)
		records = append(records, change)
	}
	kept := records[:0]
	for _, record := range records {
		if !bytes.Equal(record.Before, record.After) {
			kept = append(kept, record)
		}
	}
	return kept
}

// ChangePage is a bounded, immutable part of a revision-bound change capture.
// Through advances even across no-op transitions. A caller must retain its
// starting cursor until every page through Target has committed durably.
type ChangePage struct {
	Through uint64
	Target  uint64
	Changes []RecordChange
}

type recordTable struct {
	name, key, value string
}

func recordTables() []recordTable {
	names := append(schemaTableNames(), "debt_payment_links", "debt_payee_rules")
	tables := make([]recordTable, 0, len(names)+1)
	for _, name := range names {
		t := recordTable{name: name, key: "id", value: "data"}
		if name == "appkv" || name == "settingskv" {
			t.key, t.value = "k", "json_quote(v)"
		}
		tables = append(tables, t)
	}
	return append(tables, recordTable{name: "debt_payment_link_txns", key: "json_array(link_id,txn_id)", value: "json_array(link_id,txn_id)"})
}

func qualified(expression, prefix string) string {
	switch expression {
	case "json_quote(v)":
		return "json_quote(" + prefix + ".v)"
	case "json_array(link_id,txn_id)":
		return "json_array(" + prefix + ".link_id," + prefix + ".txn_id)"
	default:
		return prefix + "." + expression
	}
}

func (s *SQLiteStore) installChangeCapture() error {
	if _, err := s.db.Exec(`CREATE TEMP TABLE store_change_journal (
 seq INTEGER PRIMARY KEY AUTOINCREMENT, table_name TEXT NOT NULL,
 record_id TEXT NOT NULL, before_value TEXT, after_value TEXT)`); err != nil {
		return fmt.Errorf("store: create change capture: %w", err)
	}
	tables, err := tableNames(s.db)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec("CREATE TEMP TABLE store_capture_meta (pruned INTEGER NOT NULL); INSERT INTO store_capture_meta VALUES(0)"); err != nil {
		return fmt.Errorf("store: create capture watermark: %w", err)
	}
	for _, table := range recordTables() {
		if !tables[table.name] {
			continue
		}
		if err := s.installTableCapture(table); err != nil {
			return err
		}
	}
	s.captureReady = true
	return nil
}

func (s *SQLiteStore) installTableCapture(table recordTable) error {
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
		before, after, key := "NULL", "NULL", qualified(table.key, "NEW")
		if operation != "INSERT" {
			before = qualified(table.value, "OLD")
		}
		if operation != "DELETE" {
			after = qualified(table.value, "NEW")
		} else {
			key = qualified(table.key, "OLD")
		}
		when := ""
		if operation == "UPDATE" {
			when = " WHEN " + before + " IS NOT " + after
		}
		query := fmt.Sprintf(`CREATE TEMP TRIGGER IF NOT EXISTS capture_%s_%s AFTER %s ON %s%s
 BEGIN INSERT INTO store_change_journal(table_name,record_id,before_value,after_value)
 VALUES('%s',%s,%s,%s); END`, table.name, operation, operation, table.name, when, table.name, key, before, after)
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("store: capture %s %s: %w", table.name, operation, err)
		}
	}
	return nil
}

// ChangeCursor returns this store's committed high-water sequence. Unlike Rev,
// it is local to this store and includes every SQL mutation and batch receipt.
func (s *SQLiteStore) ChangeCursor() (uint64, error) {
	var cursor uint64
	err := s.db.QueryRow("SELECT COALESCE((SELECT seq FROM temp.sqlite_sequence WHERE name = ?),0)", changeJournalTable).Scan(&cursor)
	if err != nil {
		return 0, fmt.Errorf("store: read change cursor: %w", err)
	}
	return cursor, nil
}

// CaptureChanges reads at most limit transitions after after, never beyond the
// caller's fixed target. Writes arriving between pages belong to a later capture.
// Triggers participate in the originating SQL transaction, so rolled-back rows
// and half-written batches can never enter a capture.
func (s *SQLiteStore) CaptureChanges(after, target uint64, limit int) (ChangePage, error) {
	page := ChangePage{Through: after, Target: target}
	if limit <= 0 || target < after {
		return page, fmt.Errorf("store: invalid change capture range or limit")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return page, fmt.Errorf("store: begin capture page: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var high, pruned uint64
	if err := tx.QueryRow("SELECT COALESCE((SELECT seq FROM temp.sqlite_sequence WHERE name=?),0),pruned FROM store_capture_meta", changeJournalTable).Scan(&high, &pruned); err != nil {
		return page, err
	}
	if after < pruned {
		return page, ErrCaptureExpired
	}
	if target > high {
		return page, fmt.Errorf("store: capture target exceeds committed history")
	}
	rows, err := tx.Query(`SELECT seq, table_name, record_id, before_value, after_value
 FROM store_change_journal WHERE seq > ? AND seq <= ? ORDER BY seq LIMIT ?`, after, target, limit)
	if err != nil {
		return page, fmt.Errorf("store: read change capture: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c RecordChange
		var before, after sql.NullString
		if err := rows.Scan(&page.Through, &c.Table, &c.ID, &before, &after); err != nil {
			return page, fmt.Errorf("store: decode change capture: %w", err)
		}
		if before.Valid {
			c.Before = json.RawMessage(before.String)
		}
		if after.Valid {
			c.After = json.RawMessage(after.String)
		}
		if c.Table == "settings" {
			c.Before, err = redactRecordSettings(c.Before)
			if err == nil {
				c.After, err = redactRecordSettings(c.After)
			}
			if err != nil {
				return page, err
			}
		}
		page.Changes = append(page.Changes, c)
	}
	if err := rows.Err(); err != nil {
		return page, fmt.Errorf("store: iterate change capture: %w", err)
	}
	if len(page.Changes) < limit {
		page.Through = target
	}
	return page, nil
}

func redactRecordSettings(raw json.RawMessage) (json.RawMessage, error) {
	if raw == nil {
		return nil, nil
	}
	var settings Settings
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("store: decode captured settings: %w", err)
	}
	settings.OpenAIKey = ""
	return json.Marshal(settings)
}

// PruneChanges releases acknowledged capture history. Never call it before the
// durable commit callback, or while an older undo/capture consumer still needs it.
func (s *SQLiteStore) PruneChanges(through uint64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin capture pruning: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var high uint64
	if err := tx.QueryRow("SELECT COALESCE((SELECT seq FROM temp.sqlite_sequence WHERE name=?),0)", changeJournalTable).Scan(&high); err != nil {
		return err
	}
	if through > high {
		return fmt.Errorf("store: cannot prune uncommitted capture history")
	}
	if _, err := tx.Exec("DELETE FROM store_change_journal WHERE seq<=?", through); err != nil {
		return fmt.Errorf("store: prune change capture: %w", err)
	}
	if _, err := tx.Exec("UPDATE store_capture_meta SET pruned=MAX(pruned,?)", through); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit capture pruning: %w", err)
	}
	return nil
}

// ReplayChanges applies an authenticated, ordered capture atomically. Table names
// are checked against the schema allowlist. Replay itself is captured, allowing
// imports/sync/undo to use the same persistence path as ordinary edits.
func (s *SQLiteStore) ReplayChanges(changes []RecordChange) error {
	return s.replayChanges(changes, false)
}

// ReplayChangesExpected refuses to overwrite a record whose current value no
// longer matches Before. Undo uses this guard to preserve intervening edits.
func (s *SQLiteStore) ReplayChangesExpected(changes []RecordChange) error {
	return s.replayChanges(changes, true)
}

func (s *SQLiteStore) replayChanges(changes []RecordChange, expected bool) error {
	if len(changes) == 0 {
		return nil
	}
	if err := s.ensureDebtPaymentSchema(); err != nil {
		return err
	}
	tables := make(map[string]recordTable)
	for _, table := range recordTables() {
		tables[table.name] = table
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("store: begin replay: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, c := range changes {
		table, ok := tables[c.Table]
		if !ok || (c.ID == "" && c.Table != "appkv" && c.Table != "settingskv") || (c.After != nil && !json.Valid(c.After)) {
			return fmt.Errorf("store: invalid replay record %q/%q", c.Table, c.ID)
		}
		if expected {
			var current string
			var readErr error
			if c.Table == "appkv" || c.Table == "settingskv" {
				readErr = tx.QueryRow("SELECT json_quote(v) FROM "+table.name+" WHERE k=?", c.ID).Scan(&current)
			} else if c.Table == "debt_payment_link_txns" {
				return fmt.Errorf("store: membership cannot be independently undone")
			} else {
				readErr = tx.QueryRow("SELECT data FROM "+table.name+" WHERE id=?", c.ID).Scan(&current)
			}
			if readErr != nil && readErr != sql.ErrNoRows {
				return fmt.Errorf("store: read expected replay value: %w", readErr)
			}
			raw := json.RawMessage(current)
			if c.Table == "settings" && readErr == nil {
				raw, err = redactRecordSettings(raw)
				if err != nil {
					return err
				}
			}
			var compactCurrent, compactBefore bytes.Buffer
			if len(raw) > 0 {
				_ = json.Compact(&compactCurrent, raw)
			}
			if len(c.Before) > 0 {
				_ = json.Compact(&compactBefore, c.Before)
			}
			if (readErr == sql.ErrNoRows) != (c.Before == nil) || !bytes.Equal(compactCurrent.Bytes(), compactBefore.Bytes()) {
				return fmt.Errorf("store: %s/%s changed since this undo point; it was not overwritten", c.Table, c.ID)
			}
		}
		if c.Table == "settings" && c.After != nil {
			var next Settings
			if err := json.Unmarshal(c.After, &next); err != nil {
				return err
			}
			var current string
			if err := tx.QueryRow("SELECT data FROM settings WHERE id=?", c.ID).Scan(&current); err == nil {
				var prior Settings
				if err := json.Unmarshal([]byte(current), &prior); err != nil {
					return err
				}
				next.OpenAIKey = prior.OpenAIKey
			} else if err != sql.ErrNoRows {
				return err
			}
			c.After, err = json.Marshal(next)
			if err != nil {
				return err
			}
		}

		switch c.Table {
		case "appkv", "settingskv":
			if c.After == nil {
				_, err = tx.Exec("DELETE FROM "+table.name+" WHERE k = ?", c.ID)
			} else {
				var value string
				if err = json.Unmarshal(c.After, &value); err == nil {
					_, err = tx.Exec("INSERT INTO "+table.name+"(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", c.ID, value)
				}
			}
		case "debt_payment_link_txns":
			var keys []string
			if err = json.Unmarshal([]byte(c.ID), &keys); err != nil || len(keys) != 2 {
				return fmt.Errorf("store: invalid debt-payment replay key")
			}
			if c.After == nil {
				_, err = tx.Exec("DELETE FROM debt_payment_link_txns WHERE link_id=? AND txn_id=?", keys[0], keys[1])
			} else {
				_, err = tx.Exec("INSERT OR IGNORE INTO debt_payment_link_txns(link_id,txn_id) VALUES(?,?)", keys[0], keys[1])
			}
		default:
			if c.After == nil {
				_, err = tx.Exec("DELETE FROM "+table.name+" WHERE id = ?", c.ID)
			} else {
				_, err = tx.Exec("INSERT INTO "+table.name+"(id,data) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", c.ID, string(c.After))
			}
		}
		if err != nil {
			return fmt.Errorf("store: replay %s/%s: %w", c.Table, c.ID, err)
		}
		if c.Table == "debt_payment_links" && c.After != nil {
			var link domain.DebtPaymentLink
			if err := json.Unmarshal(c.After, &link); err != nil {
				return err
			}
			if _, err := tx.Exec("DELETE FROM debt_payment_link_txns WHERE link_id=?", c.ID); err != nil {
				return err
			}
			for _, txnID := range link.TransactionIDs {
				if _, err := tx.Exec("INSERT INTO debt_payment_link_txns(link_id,txn_id) VALUES(?,?)", c.ID, txnID); err != nil {
					return err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit replay: %w", err)
	}
	if len(changes) > 0 {
		mutationRev.Add(1)
	}
	return nil
}
