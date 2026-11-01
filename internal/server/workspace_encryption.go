// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

var errWorkspaceEncryptionDowngrade = errors.New("the server copy was already encrypted; CashFlux refused an unprotected replacement")

// encryptionMarkerName uses an unambiguous tenant/workspace tuple. server_meta
// keeps this monotonic policy independently of snapshot retention and clocks.
func encryptionMarkerName(userID, workspaceID string) string {
	scope, _ := json.Marshal([2]string{userID, workspaceID})
	return "workspace-encrypted:" + string(scope)
}

func markWorkspaceEncryptedTx(tx *sql.Tx, userID, workspaceID string) error {
	_, err := tx.Exec(`INSERT INTO server_meta(name, value) VALUES(?, 'true') ON CONFLICT(name) DO NOTHING`, encryptionMarkerName(userID, workspaceID))
	if err != nil {
		return fmt.Errorf("server store: mark workspace encrypted: %w", err)
	}
	return nil
}

// workspaceEncryptedTx also recognizes pre-marker installations. Both current
// and historical encrypted snapshots are evidence; device timestamps are not.
func workspaceEncryptedTx(tx *sql.Tx, userID, workspaceID string) (bool, error) {
	var marker string
	err := tx.QueryRow(`SELECT value FROM server_meta WHERE name = ?`, encryptionMarkerName(userID, workspaceID)).Scan(&marker)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("server store: read workspace encryption marker: %w", err)
	}
	rows, err := tx.Query(`SELECT dataset_json FROM snapshots WHERE user_id = ? AND workspace_id = ?
UNION ALL SELECT dataset_json FROM snapshot_history WHERE user_id = ? AND workspace_id = ?`, userID, workspaceID, userID, workspaceID)
	if err != nil {
		return false, fmt.Errorf("server store: read workspace encryption evidence: %w", err)
	}
	encrypted := false
	for rows.Next() {
		var dataset []byte
		if err := rows.Scan(&dataset); err != nil {
			_ = rows.Close()
			return false, fmt.Errorf("server store: scan workspace encryption evidence: %w", err)
		}
		encrypted = encrypted || cryptobox.IsEnvelope(dataset)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return false, fmt.Errorf("server store: iterate workspace encryption evidence: %w", err)
	}
	if err := rows.Close(); err != nil {
		return false, fmt.Errorf("server store: close workspace encryption evidence: %w", err)
	}
	if encrypted {
		return true, markWorkspaceEncryptedTx(tx, userID, workspaceID)
	}
	return false, nil
}

// checkSnapshotEncryption latches historical evidence before serving a copy.
// Commit the marker even when refusing a damaged/restored plaintext current row.
func (s *Store) checkSnapshotEncryption(snapshot Snapshot) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("server store: begin workspace encryption check: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	encrypted, err := workspaceEncryptedTx(tx, snapshot.UserID, snapshot.WorkspaceID)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("server store: commit workspace encryption check: %w", err)
	}
	if encrypted && !cryptobox.IsEnvelope(snapshot.Dataset) {
		return errWorkspaceEncryptionDowngrade
	}
	return nil
}
