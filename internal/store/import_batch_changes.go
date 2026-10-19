// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
)

func writeImportChanges(tx *sql.Tx, changes []RecordChange) error {
	// Imports can plan these effects. Keep the allowlist deliberately narrow:
	// scope, settings, credentials and unrelated records cannot enter the batch.
	allowed := map[string]bool{"transactions": true, "goals": true, "rules": true, "tasks": true, "workflowruns": true}
	inserts := map[string]*sql.Stmt{}
	defer func() {
		for _, statement := range inserts {
			_ = statement.Close()
		}
	}()
	for _, change := range CoalesceChanges(changes) {
		if !allowed[change.Table] || change.ID == "" || change.After == nil || !json.Valid(change.After) {
			return fmt.Errorf("store: invalid import effect %q/%q", change.Table, change.ID)
		}
		if change.Before == nil {
			statement := inserts[change.Table]
			if statement == nil {
				var err error
				statement, err = tx.Prepare("INSERT INTO " + change.Table + "(id,data) VALUES(?,?)")
				if err != nil {
					return fmt.Errorf("store: prepare import effects: %w", err)
				}
				inserts[change.Table] = statement
			}
			if _, err := statement.Exec(change.ID, string(change.After)); err != nil {
				return fmt.Errorf("store: insert import effect %s/%s: %w", change.Table, change.ID, err)
			}
			continue
		}
		var current string
		err := tx.QueryRow("SELECT data FROM "+change.Table+" WHERE id=?", change.ID).Scan(&current)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("store: inspect import effect %s/%s: %w", change.Table, change.ID, err)
		}
		var beforeBytes, currentBytes bytes.Buffer
		if change.Before != nil {
			if err := json.Compact(&beforeBytes, change.Before); err != nil {
				return fmt.Errorf("store: invalid import before-image: %w", err)
			}
		}
		if current != "" {
			if err := json.Compact(&currentBytes, []byte(current)); err != nil {
				return fmt.Errorf("store: invalid current import effect: %w", err)
			}
		}
		if (err == sql.ErrNoRows) != (change.Before == nil) || !bytes.Equal(beforeBytes.Bytes(), currentBytes.Bytes()) {
			return fmt.Errorf("store: import effect %s/%s changed before commit; nothing was imported", change.Table, change.ID)
		}
		if _, err := tx.Exec("INSERT INTO "+change.Table+"(id,data) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", change.ID, string(change.After)); err != nil {
			return fmt.Errorf("store: write import effect %s/%s: %w", change.Table, change.ID, err)
		}
	}
	return nil
}
