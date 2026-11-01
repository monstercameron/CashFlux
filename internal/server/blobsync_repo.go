// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type storedChange struct {
	ChangeID, DeviceID string
	Seq                int64
	Blob               []byte
}

func (s *Store) appendBlobChanges(ctx context.Context, userID, workspaceID string, changes []storedChange, now time.Time) ([]int64, error) {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("blob sync: acquire connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return nil, fmt.Errorf("blob sync: begin append: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	var deleted string
	err = conn.QueryRowContext(ctx, `SELECT deleted_at FROM ws_head WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&deleted)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("blob sync: read workspace head: %w", err)
	}
	if deleted != "" {
		return nil, errWorkspaceDeleted
	}
	if err == sql.ErrNoRows {
		if _, err := conn.ExecContext(ctx, `INSERT INTO ws_head(user_id,workspace_id,next_seq) VALUES(?,?,1)`, userID, workspaceID); err != nil {
			return nil, fmt.Errorf("blob sync: create workspace head: %w", err)
		}
	}
	var nextSeq int64
	if err := conn.QueryRowContext(ctx, `SELECT next_seq FROM ws_head WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&nextSeq); err != nil {
		return nil, fmt.Errorf("blob sync: read next sequence: %w", err)
	}
	seqs := make([]int64, len(changes))
	advanced := false
	for i, change := range changes {
		var seq int64
		err := conn.QueryRowContext(ctx, `SELECT seq FROM change_log WHERE user_id=? AND workspace_id=? AND change_id=?`, userID, workspaceID, change.ChangeID).Scan(&seq)
		if err == nil {
			seqs[i] = seq
			continue
		}
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("blob sync: check idempotency key: %w", err)
		}
		seq = nextSeq
		if _, err := conn.ExecContext(ctx, `INSERT INTO change_log(user_id,workspace_id,seq,change_id,device_id,size,created_at,blob) VALUES(?,?,?,?,?,?,?,?)`, userID, workspaceID, seq, change.ChangeID, change.DeviceID, len(change.Blob), now.UTC().Format(time.RFC3339Nano), change.Blob); err != nil {
			return nil, fmt.Errorf("blob sync: append change: %w", err)
		}
		nextSeq++
		advanced = true
		seqs[i] = seq
	}
	if advanced {
		if _, err := conn.ExecContext(ctx, `UPDATE ws_head SET next_seq=? WHERE user_id=? AND workspace_id=?`, nextSeq, userID, workspaceID); err != nil {
			return nil, fmt.Errorf("blob sync: advance sequence: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return nil, fmt.Errorf("blob sync: commit append: %w", err)
	}
	committed = true
	return seqs, nil
}

var errWorkspaceDeleted = fmt.Errorf("workspace has been deleted")

func (s *Store) listBlobChanges(ctx context.Context, userID, workspaceID string, since int64, limit int) ([]storedChange, int64, error) {
	var head int64
	var deleted string
	err := s.db.QueryRowContext(ctx, `SELECT next_seq-1,deleted_at FROM ws_head WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&head, &deleted)
	if err == sql.ErrNoRows {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("blob sync: read head: %w", err)
	}
	if deleted != "" {
		return nil, head, errWorkspaceDeleted
	}
	rows, err := s.db.QueryContext(ctx, `SELECT seq,change_id,device_id,blob FROM change_log WHERE user_id=? AND workspace_id=? AND seq>? ORDER BY seq LIMIT ?`, userID, workspaceID, since, limit)
	if err != nil {
		return nil, head, fmt.Errorf("blob sync: list changes: %w", err)
	}
	defer rows.Close()
	var out []storedChange
	for rows.Next() {
		var c storedChange
		if err := rows.Scan(&c.Seq, &c.ChangeID, &c.DeviceID, &c.Blob); err != nil {
			return nil, head, fmt.Errorf("blob sync: scan change: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, head, fmt.Errorf("blob sync: iterate changes: %w", err)
	}
	return out, head, nil
}

func (s *Store) putBlobSnapshot(ctx context.Context, userID, workspaceID string, covers int64, blob []byte, retention time.Duration, now time.Time) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("blob sync: begin snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var head int64
	var deleted string
	err = tx.QueryRowContext(ctx, `SELECT next_seq-1,deleted_at FROM ws_head WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&head, &deleted)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("blob sync: read snapshot head: %w", err)
	}
	if deleted != "" {
		return false, errWorkspaceDeleted
	}
	var max int64
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(covers_seq),0) FROM ws_snapshot WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&max)
	if err != nil {
		return false, fmt.Errorf("blob sync: read snapshot cursor: %w", err)
	}
	if covers <= max || covers > head {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO ws_snapshot(user_id,workspace_id,covers_seq,created_at,blob) VALUES(?,?,?,?,?)`, userID, workspaceID, covers, now.UTC().Format(time.RFC3339Nano), blob); err != nil {
		return false, fmt.Errorf("blob sync: put snapshot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ws_snapshot WHERE user_id=? AND workspace_id=? AND covers_seq NOT IN (SELECT covers_seq FROM ws_snapshot WHERE user_id=? AND workspace_id=? ORDER BY covers_seq DESC LIMIT 3)`, userID, workspaceID, userID, workspaceID); err != nil {
		return false, fmt.Errorf("blob sync: prune snapshots: %w", err)
	}
	if retention > 0 {
		cutoff := now.Add(-retention).UTC().Format(time.RFC3339Nano)
		if _, err := tx.ExecContext(ctx, `DELETE FROM change_log WHERE user_id=? AND workspace_id=? AND seq<=? AND created_at<?`, userID, workspaceID, covers, cutoff); err != nil {
			return false, fmt.Errorf("blob sync: prune changes: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("blob sync: commit snapshot: %w", err)
	}
	return true, nil
}

func (s *Store) getBlobSnapshot(ctx context.Context, userID, workspaceID string) (int64, []byte, bool, error) {
	var n int64
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT covers_seq,blob FROM ws_snapshot WHERE user_id=? AND workspace_id=? ORDER BY covers_seq DESC LIMIT 1`, userID, workspaceID).Scan(&n, &b)
	if err == sql.ErrNoRows {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, fmt.Errorf("blob sync: get snapshot: %w", err)
	}
	return n, b, true, nil
}

func (s *Store) getBlobKeyring(ctx context.Context, userID, workspaceID string) (int64, []byte, bool, error) {
	var v int64
	var b []byte
	err := s.db.QueryRowContext(ctx, `SELECT version,blob FROM keyring WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&v, &b)
	if err == sql.ErrNoRows {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, fmt.Errorf("blob sync: get keyring: %w", err)
	}
	return v, b, true, nil
}

func (s *Store) putBlobKeyring(ctx context.Context, userID, workspaceID string, expected int64, blob []byte) (int64, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("blob sync: begin keyring: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT version FROM keyring WHERE user_id=? AND workspace_id=?`, userID, workspaceID).Scan(&current)
	if err != nil && err != sql.ErrNoRows {
		return 0, false, fmt.Errorf("blob sync: read keyring version: %w", err)
	}
	if err == sql.ErrNoRows {
		current = 0
	}
	if current != expected {
		return current, false, nil
	}
	next := current + 1
	if current == 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO keyring(user_id,workspace_id,version,blob) VALUES(?,?,?,?)`, userID, workspaceID, next, blob)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE keyring SET version=?,blob=? WHERE user_id=? AND workspace_id=? AND version=?`, next, blob, userID, workspaceID, current)
	}
	if err != nil {
		return 0, false, fmt.Errorf("blob sync: update keyring: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("blob sync: commit keyring: %w", err)
	}
	return next, true, nil
}

func (s *Store) deleteBlobWorkspace(ctx context.Context, userID, workspaceID string, now time.Time) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE ws_head SET deleted_at=? WHERE user_id=? AND workspace_id=? AND deleted_at=''`, now.UTC().Format(time.RFC3339Nano), userID, workspaceID)
	if err != nil {
		return false, fmt.Errorf("blob sync: tombstone workspace: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) sweepPairingRelay(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pairing_relay WHERE expires_at<=?`, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("blob sync: sweep pairings: %w", err)
	}
	return nil
}
