package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// User is an authenticated backend account.
type User struct {
	ID        string
	Provider  string
	Subject   string
	Email     string
	CreatedAt time.Time
}

// Workspace is the server-side registry row for one synced client workspace.
type Workspace struct {
	ID        string
	UserID    string
	Name      string
	Color     string
	Sort      int
	Deleted   bool
	Version   int64
	UpdatedAt time.Time
	DeviceID  string
}

// Snapshot is an opaque gzipped dataset payload for one workspace version.
type Snapshot struct {
	WorkspaceID string
	Dataset     []byte
	Version     int64
	UpdatedAt   time.Time
}

// Blob is content-addressed artifact metadata. Bytes live on disk by hash.
type Blob struct {
	Hash      string
	Size      int64
	Mime      string
	Name      string
	CreatedAt time.Time
}

// UpsertUser stores an OAuth identity, keyed by provider+subject.
func (s *Store) UpsertUser(u User) error {
	if strings.TrimSpace(u.ID) == "" || strings.TrimSpace(u.Provider) == "" || strings.TrimSpace(u.Subject) == "" {
		return fmt.Errorf("server store: user id, provider, and subject are required")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
INSERT INTO users(id, provider, subject, email, created_at)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(provider, subject) DO UPDATE SET email = excluded.email`,
		u.ID, u.Provider, u.Subject, u.Email, formatTime(u.CreatedAt))
	if err != nil {
		return fmt.Errorf("server store: upsert user: %w", err)
	}
	return nil
}

// PutWorkspace inserts or replaces a workspace registry row.
func (s *Store) PutWorkspace(w Workspace) error {
	if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.UserID) == "" || strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("server store: workspace id, user id, and name are required")
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
INSERT INTO workspaces(id, user_id, name, color, sort, deleted, version, updated_at, device_id)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  user_id = excluded.user_id,
  name = excluded.name,
  color = excluded.color,
  sort = excluded.sort,
  deleted = excluded.deleted,
  version = excluded.version,
  updated_at = excluded.updated_at,
  device_id = excluded.device_id`,
		w.ID, w.UserID, w.Name, w.Color, w.Sort, boolInt(w.Deleted), w.Version, formatTime(w.UpdatedAt), w.DeviceID)
	if err != nil {
		return fmt.Errorf("server store: put workspace: %w", err)
	}
	return nil
}

// ListWorkspaces returns a user's workspaces ordered for display.
func (s *Store) ListWorkspaces(userID string, includeDeleted bool) ([]Workspace, error) {
	query := `SELECT id, user_id, name, color, sort, deleted, version, updated_at, device_id FROM workspaces WHERE user_id = ?`
	args := []any{userID}
	if !includeDeleted {
		query += ` AND deleted = 0`
	}
	query += ` ORDER BY sort, name, id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("server store: list workspaces: %w", err)
	}
	defer rows.Close()

	var out []Workspace
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: list workspaces rows: %w", err)
	}
	return out, nil
}

// GetWorkspace returns one workspace by id, scoped to a user.
func (s *Store) GetWorkspace(userID, workspaceID string) (Workspace, bool, error) {
	row := s.db.QueryRow(`
SELECT id, user_id, name, color, sort, deleted, version, updated_at, device_id
FROM workspaces WHERE user_id = ? AND id = ?`, userID, workspaceID)
	w, err := scanWorkspace(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, false, nil
	}
	if err != nil {
		return Workspace{}, false, err
	}
	return w, true, nil
}

// SoftDeleteWorkspace marks a user's workspace as deleted.
func (s *Store) SoftDeleteWorkspace(userID, workspaceID string, updatedAt time.Time, deviceID string) (bool, error) {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`
UPDATE workspaces SET deleted = 1, updated_at = ?, device_id = ?
WHERE user_id = ? AND id = ?`, formatTime(updatedAt), deviceID, userID, workspaceID)
	if err != nil {
		return false, fmt.Errorf("server store: soft delete workspace: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: soft delete rows affected: %w", err)
	}
	return n > 0, nil
}

// PutSnapshot stores the current dataset snapshot, retaining the previous
// current snapshot in history and trimming that history to historyLimit entries.
func (s *Store) PutSnapshot(snapshot Snapshot, maxBytes, historyLimit int) error {
	if strings.TrimSpace(snapshot.WorkspaceID) == "" {
		return fmt.Errorf("server store: snapshot workspace id is required")
	}
	if maxBytes > 0 && len(snapshot.Dataset) > maxBytes {
		return fmt.Errorf("server store: snapshot dataset is %d bytes, exceeds limit %d", len(snapshot.Dataset), maxBytes)
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current Snapshot
	var updated string
	row := tx.QueryRow(`SELECT workspace_id, dataset_json, version, updated_at FROM snapshots WHERE workspace_id = ?`, snapshot.WorkspaceID)
	err = row.Scan(&current.WorkspaceID, &current.Dataset, &current.Version, &updated)
	if err == nil {
		current.UpdatedAt, err = parseTime(updated)
		if err != nil {
			return fmt.Errorf("server store: parse current snapshot time: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO snapshot_history(workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?)`,
			current.WorkspaceID, current.Dataset, current.Version, formatTime(current.UpdatedAt)); err != nil {
			return fmt.Errorf("server store: snapshot history: %w", err)
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("server store: read current snapshot: %w", err)
	}

	if _, err := tx.Exec(`
INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(workspace_id) DO UPDATE SET
  dataset_json = excluded.dataset_json,
  version = excluded.version,
  updated_at = excluded.updated_at`,
		snapshot.WorkspaceID, snapshot.Dataset, snapshot.Version, formatTime(snapshot.UpdatedAt)); err != nil {
		return fmt.Errorf("server store: put snapshot: %w", err)
	}
	if err := trimSnapshotHistory(tx, snapshot.WorkspaceID, historyLimit); err != nil {
		return err
	}
	return tx.Commit()
}

// GetSnapshot returns the current dataset snapshot for a workspace.
func (s *Store) GetSnapshot(workspaceID string) (Snapshot, bool, error) {
	row := s.db.QueryRow(`SELECT workspace_id, dataset_json, version, updated_at FROM snapshots WHERE workspace_id = ?`, workspaceID)
	snapshot, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	return snapshot, true, nil
}

// SnapshotHistory returns retained prior snapshots newest-first.
func (s *Store) SnapshotHistory(workspaceID string, limit int) ([]Snapshot, error) {
	query := `SELECT workspace_id, dataset_json, version, updated_at FROM snapshot_history WHERE workspace_id = ? ORDER BY version DESC, id DESC`
	args := []any{workspaceID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("server store: snapshot history: %w", err)
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		snapshot, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: snapshot history rows: %w", err)
	}
	return out, nil
}

// PutBlob stores bytes under a sha256 content-addressed path and records metadata.
func (s *Store) PutBlob(root string, data []byte, mime, name string, maxBytes int64) (Blob, error) {
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return Blob{}, fmt.Errorf("server store: blob is %d bytes, exceeds limit %d", len(data), maxBytes)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	blob := Blob{
		Hash:      hash,
		Size:      int64(len(data)),
		Mime:      strings.TrimSpace(mime),
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now().UTC(),
	}
	path := blobPath(root, hash)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Blob{}, fmt.Errorf("server store: blob mkdir: %w", err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return Blob{}, fmt.Errorf("server store: blob write: %w", err)
		}
	} else if err != nil {
		return Blob{}, fmt.Errorf("server store: blob stat: %w", err)
	}
	if _, err := s.db.Exec(`
INSERT INTO blobs(hash, size, mime, created_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(hash) DO UPDATE SET size = excluded.size, mime = excluded.mime`,
		blob.Hash, blob.Size, blob.Mime, formatTime(blob.CreatedAt)); err != nil {
		return Blob{}, fmt.Errorf("server store: put blob metadata: %w", err)
	}
	return blob, nil
}

// ReadBlob reads content-addressed blob bytes from disk.
func (s *Store) ReadBlob(root, hash string) ([]byte, error) {
	if _, ok, err := s.GetBlob(hash); err != nil {
		return nil, err
	} else if !ok {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(blobPath(root, hash))
	if err != nil {
		return nil, fmt.Errorf("server store: blob read: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("server store: blob hash mismatch")
	}
	return data, nil
}

// GetBlob returns stored blob metadata.
func (s *Store) GetBlob(hash string) (Blob, bool, error) {
	row := s.db.QueryRow(`SELECT hash, size, mime, created_at FROM blobs WHERE hash = ?`, hash)
	blob, err := scanBlob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Blob{}, false, nil
	}
	if err != nil {
		return Blob{}, false, err
	}
	return blob, true, nil
}

// LinkWorkspaceBlob records that a workspace snapshot references a blob hash.
func (s *Store) LinkWorkspaceBlob(workspaceID, hash string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(hash) == "" {
		return fmt.Errorf("server store: workspace id and blob hash are required")
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO workspace_blobs(workspace_id, hash) VALUES(?, ?)`, workspaceID, hash); err != nil {
		return fmt.Errorf("server store: link workspace blob: %w", err)
	}
	return nil
}

// WorkspaceBlobs returns blob metadata linked to a workspace.
func (s *Store) WorkspaceBlobs(workspaceID string) ([]Blob, error) {
	rows, err := s.db.Query(`
SELECT b.hash, b.size, b.mime, b.created_at
FROM blobs b
JOIN workspace_blobs wb ON wb.hash = b.hash
WHERE wb.workspace_id = ?
ORDER BY b.hash`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("server store: workspace blobs: %w", err)
	}
	defer rows.Close()
	var out []Blob
	for rows.Next() {
		blob, err := scanBlob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, blob)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: workspace blobs rows: %w", err)
	}
	return out, nil
}

// SweepUnreferencedBlobs deletes metadata and files for blobs no workspace references.
func (s *Store) SweepUnreferencedBlobs(root string) (int, error) {
	rows, err := s.db.Query(`
SELECT b.hash, b.size, b.mime, b.created_at
FROM blobs b
LEFT JOIN workspace_blobs wb ON wb.hash = b.hash
WHERE wb.hash IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("server store: unreferenced blobs: %w", err)
	}
	defer rows.Close()
	var blobs []Blob
	for rows.Next() {
		blob, err := scanBlob(rows)
		if err != nil {
			return 0, err
		}
		blobs = append(blobs, blob)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("server store: unreferenced blob rows: %w", err)
	}
	for _, blob := range blobs {
		_ = os.Remove(blobPath(root, blob.Hash))
		if _, err := s.db.Exec(`DELETE FROM blobs WHERE hash = ?`, blob.Hash); err != nil {
			return 0, fmt.Errorf("server store: delete blob metadata: %w", err)
		}
	}
	return len(blobs), nil
}

// PutAIKey encrypts and stores a user's provider key with AES-GCM.
func (s *Store) PutAIKey(userID, provider, key string, masterKey []byte) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(provider) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("server store: user id, provider, and key are required")
	}
	gcm, err := aesGCM(masterKey)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("server store: ai key nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(key), []byte(userID+"|"+provider))
	if _, err := s.db.Exec(`
INSERT INTO ai_keys(user_id, provider, ciphertext, nonce)
VALUES(?, ?, ?, ?)
ON CONFLICT(user_id, provider) DO UPDATE SET ciphertext = excluded.ciphertext, nonce = excluded.nonce`,
		userID, provider, ciphertext, nonce); err != nil {
		return fmt.Errorf("server store: put ai key: %w", err)
	}
	return nil
}

// GetAIKey decrypts a user's provider key.
func (s *Store) GetAIKey(userID, provider string, masterKey []byte) (string, bool, error) {
	gcm, err := aesGCM(masterKey)
	if err != nil {
		return "", false, err
	}
	var ciphertext, nonce []byte
	err = s.db.QueryRow(`SELECT ciphertext, nonce FROM ai_keys WHERE user_id = ? AND provider = ?`, userID, provider).Scan(&ciphertext, &nonce)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("server store: get ai key: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(userID+"|"+provider))
	if err != nil {
		return "", false, fmt.Errorf("server store: decrypt ai key: %w", err)
	}
	return string(plaintext), true, nil
}

func aesGCM(masterKey []byte) (cipher.AEAD, error) {
	if !validAESKeyLength(len(masterKey)) {
		return nil, fmt.Errorf("server store: master key must be 16, 24, or 32 bytes")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("server store: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("server store: aes-gcm: %w", err)
	}
	return gcm, nil
}

func trimSnapshotHistory(tx *sql.Tx, workspaceID string, limit int) error {
	if limit <= 0 {
		if _, err := tx.Exec(`DELETE FROM snapshot_history WHERE workspace_id = ?`, workspaceID); err != nil {
			return fmt.Errorf("server store: trim snapshot history: %w", err)
		}
		return nil
	}
	_, err := tx.Exec(`
DELETE FROM snapshot_history
WHERE workspace_id = ?
  AND id NOT IN (
    SELECT id FROM snapshot_history
    WHERE workspace_id = ?
    ORDER BY version DESC, id DESC
    LIMIT ?
  )`, workspaceID, workspaceID, limit)
	if err != nil {
		return fmt.Errorf("server store: trim snapshot history: %w", err)
	}
	return nil
}

type workspaceScanner interface {
	Scan(dest ...any) error
}

type snapshotScanner interface {
	Scan(dest ...any) error
}

type blobScanner interface {
	Scan(dest ...any) error
}

func scanWorkspace(row workspaceScanner) (Workspace, error) {
	var w Workspace
	var deleted int
	var updated string
	if err := row.Scan(&w.ID, &w.UserID, &w.Name, &w.Color, &w.Sort, &deleted, &w.Version, &updated, &w.DeviceID); err != nil {
		return Workspace{}, err
	}
	t, err := parseTime(updated)
	if err != nil {
		return Workspace{}, fmt.Errorf("server store: parse workspace updated_at: %w", err)
	}
	w.Deleted = deleted != 0
	w.UpdatedAt = t
	return w, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

func scanSnapshot(row snapshotScanner) (Snapshot, error) {
	var snapshot Snapshot
	var updated string
	if err := row.Scan(&snapshot.WorkspaceID, &snapshot.Dataset, &snapshot.Version, &updated); err != nil {
		return Snapshot{}, err
	}
	t, err := parseTime(updated)
	if err != nil {
		return Snapshot{}, fmt.Errorf("server store: parse snapshot updated_at: %w", err)
	}
	snapshot.UpdatedAt = t
	return snapshot, nil
}

func scanBlob(row blobScanner) (Blob, error) {
	var blob Blob
	var created string
	if err := row.Scan(&blob.Hash, &blob.Size, &blob.Mime, &created); err != nil {
		return Blob{}, err
	}
	t, err := parseTime(created)
	if err != nil {
		return Blob{}, fmt.Errorf("server store: parse blob created_at: %w", err)
	}
	blob.CreatedAt = t
	return blob, nil
}

func blobPath(root, hash string) string {
	if len(hash) < 4 {
		return filepath.Join(root, hash)
	}
	return filepath.Join(root, hash[:2], hash[2:4], hash)
}
