package server

import (
	"context"
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

var errPayloadTooLarge = errors.New("server store: payload too large")

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

// Usage is a per-user daily server usage counter.
type Usage struct {
	UserID   string
	Day      string
	Requests int64
	Tokens   int64
}

// Subscription is the server-side Stripe subscription state for one Cloud user.
type Subscription struct {
	UserID             string
	StripeCustomer     string
	StripeSubscription string
	Status             string
	Plan               string
	CurrentPeriodEnd   time.Time
	TrialEnd           time.Time
	UpdatedAt          time.Time
}

// AccountExport is the self-serve server-side data bundle for a user.
type AccountExport struct {
	ExportedAt      time.Time    `json:"exportedAt"`
	User            User         `json:"user"`
	Workspaces      []Workspace  `json:"workspaces"`
	Snapshots       []Snapshot   `json:"snapshots"`
	Blobs           []Blob       `json:"blobs"`
	Usage           []Usage      `json:"usage"`
	AIKeyProviders  []string     `json:"aiKeyProviders"`
	AuditEvents     []AuditEvent `json:"auditEvents"`
	RefreshSessions int          `json:"refreshSessions"`
}

// AuditEvent is a security-relevant, append-only backend event.
type AuditEvent struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	ActorID      string    `json:"actorId"`
	Action       string    `json:"action"`
	TargetType   string    `json:"targetType"`
	TargetID     string    `json:"targetId"`
	IP           string    `json:"ip,omitempty"`
	RequestID    string    `json:"requestId,omitempty"`
	PreviousHash string    `json:"previousHash"`
	Hash         string    `json:"hash"`
}

// UpsertUser stores an OAuth identity, keyed by provider+subject.
func (s *Store) UpsertUser(u User) error {
	if strings.TrimSpace(u.ID) == "" || strings.TrimSpace(u.Provider) == "" || strings.TrimSpace(u.Subject) == "" {
		return fmt.Errorf("server store: user id, provider, and subject are required")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	defer s.observeDB("UpsertUser", time.Now())
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

func (s *Store) GetUserByID(userID string) (User, bool, error) {
	defer s.observeDB("GetUserByID", time.Now())
	var u User
	var created string
	err := s.db.QueryRow(`SELECT id, provider, subject, email, created_at FROM users WHERE id = ?`, userID).
		Scan(&u.ID, &u.Provider, &u.Subject, &u.Email, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("server store: get user: %w", err)
	}
	u.CreatedAt, err = parseTime(created)
	if err != nil {
		return User{}, false, fmt.Errorf("server store: parse user time: %w", err)
	}
	return u, true, nil
}

// ExportAccount returns the user's server-side Cloud data without decrypted secrets or blob bytes.
func (s *Store) ExportAccount(userID string, exportedAt time.Time) (AccountExport, bool, error) {
	if strings.TrimSpace(userID) == "" {
		return AccountExport{}, false, fmt.Errorf("server store: user id is required")
	}
	user, ok, err := s.GetUserByID(userID)
	if err != nil || !ok {
		return AccountExport{}, ok, err
	}
	out := AccountExport{ExportedAt: exportedAt.UTC(), User: user}
	if out.Workspaces, err = s.ListWorkspaces(userID, true); err != nil {
		return AccountExport{}, false, err
	}
	if out.Snapshots, err = s.exportSnapshots(userID); err != nil {
		return AccountExport{}, false, err
	}
	if out.Blobs, err = s.exportBlobs(userID); err != nil {
		return AccountExport{}, false, err
	}
	if out.Usage, err = s.exportUsage(userID); err != nil {
		return AccountExport{}, false, err
	}
	if out.AIKeyProviders, err = s.exportAIKeyProviders(userID); err != nil {
		return AccountExport{}, false, err
	}
	if out.AuditEvents, err = s.exportAuditEvents(userID); err != nil {
		return AccountExport{}, false, err
	}
	if out.RefreshSessions, err = s.exportRefreshSessionCount(userID); err != nil {
		return AccountExport{}, false, err
	}
	return out, true, nil
}

// DeleteAccount purges the user's relational server data. Blob files are removed by a follow-up GC sweep.
func (s *Store) DeleteAccount(userID string) (bool, error) {
	if strings.TrimSpace(userID) == "" {
		return false, fmt.Errorf("server store: user id is required")
	}
	defer s.observeDB("DeleteAccount", time.Now())
	result, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return false, fmt.Errorf("server store: delete account: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("server store: delete account rows: %w", err)
	}
	return affected > 0, nil
}

type RefreshSession struct {
	JTI       string
	FamilyID  string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    time.Time
	RevokedAt time.Time
}

func (s *Store) PutRefreshSession(session RefreshSession) error {
	if strings.TrimSpace(session.JTI) == "" || strings.TrimSpace(session.FamilyID) == "" || strings.TrimSpace(session.UserID) == "" || strings.TrimSpace(session.TokenHash) == "" || session.ExpiresAt.IsZero() {
		return fmt.Errorf("server store: refresh session jti, family, user, hash, and expiry are required")
	}
	defer s.observeDB("PutRefreshSession", time.Now())
	_, err := s.db.Exec(`
INSERT INTO refresh_tokens(jti, family_id, user_id, token_hash, expires_at, used_at, revoked_at)
VALUES(?, ?, ?, ?, ?, '', '')`,
		session.JTI, session.FamilyID, session.UserID, session.TokenHash, formatTime(session.ExpiresAt.UTC()))
	if err != nil {
		return fmt.Errorf("server store: put refresh session: %w", err)
	}
	return nil
}

func (s *Store) ConsumeRefreshSession(jti, tokenHash string, now time.Time) (RefreshSession, bool, error) {
	if strings.TrimSpace(jti) == "" || strings.TrimSpace(tokenHash) == "" {
		return RefreshSession{}, false, fmt.Errorf("server store: refresh jti and hash are required")
	}
	defer s.observeDB("ConsumeRefreshSession", time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return RefreshSession{}, false, fmt.Errorf("server store: begin consume refresh session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRow(`
SELECT jti, family_id, user_id, token_hash, expires_at, used_at, revoked_at
FROM refresh_tokens
WHERE jti = ?`, jti)
	session, err := scanRefreshSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return RefreshSession{}, false, nil
	}
	if err != nil {
		return RefreshSession{}, false, err
	}
	if session.TokenHash != tokenHash || !session.UsedAt.IsZero() || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return session, false, tx.Commit()
	}
	if _, err := tx.Exec(`UPDATE refresh_tokens SET used_at = ? WHERE jti = ?`, formatTime(now.UTC()), jti); err != nil {
		return RefreshSession{}, false, fmt.Errorf("server store: mark refresh session used: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return RefreshSession{}, false, fmt.Errorf("server store: commit refresh session: %w", err)
	}
	session.UsedAt = now.UTC()
	return session, true, nil
}

func (s *Store) RevokeRefreshSessionFamily(familyID string, now time.Time) error {
	if strings.TrimSpace(familyID) == "" {
		return fmt.Errorf("server store: refresh family is required")
	}
	defer s.observeDB("RevokeRefreshSessionFamily", time.Now())
	if _, err := s.db.Exec(`UPDATE refresh_tokens SET revoked_at = ? WHERE family_id = ? AND revoked_at = ''`, formatTime(now.UTC()), familyID); err != nil {
		return fmt.Errorf("server store: revoke refresh family: %w", err)
	}
	return nil
}

// AppendAuditEvent stores a security-relevant event and links it to the previous
// event hash. Payloads intentionally carry ids and metadata, never secrets.
func (s *Store) AppendAuditEvent(event AuditEvent) (AuditEvent, error) {
	if strings.TrimSpace(event.ActorID) == "" || strings.TrimSpace(event.Action) == "" || strings.TrimSpace(event.TargetType) == "" {
		return AuditEvent{}, fmt.Errorf("server store: audit actor, action, and target type are required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.ActorID = strings.TrimSpace(event.ActorID)
	event.Action = strings.TrimSpace(event.Action)
	event.TargetType = strings.TrimSpace(event.TargetType)
	event.TargetID = strings.TrimSpace(event.TargetID)
	event.IP = strings.TrimSpace(event.IP)
	event.RequestID = strings.TrimSpace(event.RequestID)

	defer s.observeDB("AppendAuditEvent", time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return AuditEvent{}, fmt.Errorf("server store: begin audit event: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var previousHash string
	err = tx.QueryRow(`SELECT hash FROM audit_events ORDER BY id DESC LIMIT 1`).Scan(&previousHash)
	if errors.Is(err, sql.ErrNoRows) {
		previousHash = ""
	} else if err != nil {
		return AuditEvent{}, fmt.Errorf("server store: read previous audit hash: %w", err)
	}
	event.PreviousHash = previousHash
	event.Hash = auditEventHash(event)
	res, err := tx.Exec(`
INSERT INTO audit_events(timestamp, actor_id, action, target_type, target_id, ip, request_id, previous_hash, hash)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		formatTime(event.Timestamp), event.ActorID, event.Action, event.TargetType, event.TargetID, event.IP, event.RequestID, event.PreviousHash, event.Hash)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("server store: append audit event: %w", err)
	}
	event.ID, err = res.LastInsertId()
	if err != nil {
		return AuditEvent{}, fmt.Errorf("server store: audit event id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return AuditEvent{}, fmt.Errorf("server store: commit audit event: %w", err)
	}
	return event, nil
}

// ListAuditEvents returns audit events after the given id, capped by limit.
func (s *Store) ListAuditEvents(afterID int64, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	defer s.observeDB("ListAuditEvents", time.Now())
	rows, err := s.db.Query(`
SELECT id, timestamp, actor_id, action, target_type, target_id, ip, request_id, previous_hash, hash
FROM audit_events
WHERE id > ?
ORDER BY id
LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("server store: list audit events: %w", err)
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: list audit rows: %w", err)
	}
	return events, nil
}

// PruneAuditEventsBefore removes audit events older than before and returns the rows deleted.
func (s *Store) PruneAuditEventsBefore(ctx context.Context, before time.Time) (int64, error) {
	defer s.observeDB("PruneAuditEventsBefore", time.Now())
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_events WHERE timestamp < ?`, formatTime(before.UTC()))
	if err != nil {
		return 0, fmt.Errorf("server store: prune audit events: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("server store: audit prune rows: %w", err)
	}
	return deleted, nil
}

func auditEventHash(event AuditEvent) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		event.PreviousHash,
		formatTime(event.Timestamp),
		event.ActorID,
		event.Action,
		event.TargetType,
		event.TargetID,
		event.IP,
		event.RequestID,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

type auditScanner interface {
	Scan(dest ...any) error
}

type refreshSessionScanner interface {
	Scan(dest ...any) error
}

type subscriptionScanner interface {
	Scan(dest ...any) error
}

func scanRefreshSession(row refreshSessionScanner) (RefreshSession, error) {
	var session RefreshSession
	var expiresAt, usedAt, revokedAt string
	if err := row.Scan(&session.JTI, &session.FamilyID, &session.UserID, &session.TokenHash, &expiresAt, &usedAt, &revokedAt); err != nil {
		return RefreshSession{}, fmt.Errorf("server store: scan refresh session: %w", err)
	}
	var err error
	session.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return RefreshSession{}, fmt.Errorf("server store: parse refresh expiry: %w", err)
	}
	if strings.TrimSpace(usedAt) != "" {
		session.UsedAt, err = parseTime(usedAt)
		if err != nil {
			return RefreshSession{}, fmt.Errorf("server store: parse refresh used time: %w", err)
		}
	}
	if strings.TrimSpace(revokedAt) != "" {
		session.RevokedAt, err = parseTime(revokedAt)
		if err != nil {
			return RefreshSession{}, fmt.Errorf("server store: parse refresh revoked time: %w", err)
		}
	}
	return session, nil
}

func scanAuditEvent(row auditScanner) (AuditEvent, error) {
	var event AuditEvent
	var timestamp string
	if err := row.Scan(&event.ID, &timestamp, &event.ActorID, &event.Action, &event.TargetType, &event.TargetID, &event.IP, &event.RequestID, &event.PreviousHash, &event.Hash); err != nil {
		return AuditEvent{}, fmt.Errorf("server store: scan audit event: %w", err)
	}
	var err error
	event.Timestamp, err = parseTime(timestamp)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("server store: parse audit time: %w", err)
	}
	return event, nil
}

func scanSubscription(row subscriptionScanner) (Subscription, bool, error) {
	var sub Subscription
	var currentPeriodEnd, trialEnd, updatedAt string
	err := row.Scan(
		&sub.UserID,
		&sub.StripeCustomer,
		&sub.StripeSubscription,
		&sub.Status,
		&sub.Plan,
		&currentPeriodEnd,
		&trialEnd,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, false, nil
	}
	if err != nil {
		return Subscription{}, false, fmt.Errorf("server store: scan subscription: %w", err)
	}
	var parseErr error
	if sub.CurrentPeriodEnd, parseErr = parseOptionalTime(currentPeriodEnd); parseErr != nil {
		return Subscription{}, false, fmt.Errorf("server store: parse subscription current_period_end: %w", parseErr)
	}
	if sub.TrialEnd, parseErr = parseOptionalTime(trialEnd); parseErr != nil {
		return Subscription{}, false, fmt.Errorf("server store: parse subscription trial_end: %w", parseErr)
	}
	if sub.UpdatedAt, parseErr = parseTime(updatedAt); parseErr != nil {
		return Subscription{}, false, fmt.Errorf("server store: parse subscription updated_at: %w", parseErr)
	}
	return sub, true, nil
}

// PutWorkspace inserts or replaces a workspace registry row.
func (s *Store) PutWorkspace(w Workspace) error {
	if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.UserID) == "" || strings.TrimSpace(w.Name) == "" {
		return fmt.Errorf("server store: workspace id, user id, and name are required")
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = time.Now().UTC()
	}
	defer s.observeDB("PutWorkspace", time.Now())
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
	defer s.observeDB("ListWorkspaces", time.Now())
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
	defer s.observeDB("GetWorkspace", time.Now())
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

func (s *Store) WorkspaceOwner(workspaceID string) (string, bool, error) {
	defer s.observeDB("WorkspaceOwner", time.Now())
	var userID string
	err := s.db.QueryRow(`SELECT user_id FROM workspaces WHERE id = ?`, workspaceID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("server store: workspace owner: %w", err)
	}
	return userID, true, nil
}

// SoftDeleteWorkspace marks a user's workspace as deleted.
func (s *Store) SoftDeleteWorkspace(userID, workspaceID string, updatedAt time.Time, deviceID string) (bool, error) {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	defer s.observeDB("SoftDeleteWorkspace", time.Now())
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
		return fmt.Errorf("%w: snapshot dataset is %d bytes, exceeds limit %d", errPayloadTooLarge, len(snapshot.Dataset), maxBytes)
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}
	defer s.observeDB("PutSnapshot", time.Now())
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
	defer s.observeDB("GetSnapshot", time.Now())
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
	defer s.observeDB("SnapshotHistory", time.Now())
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

// PruneSnapshotHistoryBefore removes retained prior snapshots older than before.
func (s *Store) PruneSnapshotHistoryBefore(ctx context.Context, before time.Time) (int64, error) {
	defer s.observeDB("PruneSnapshotHistoryBefore", time.Now())
	res, err := s.db.ExecContext(ctx, `DELETE FROM snapshot_history WHERE updated_at < ?`, formatTime(before.UTC()))
	if err != nil {
		return 0, fmt.Errorf("server store: prune snapshot history: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("server store: snapshot prune rows: %w", err)
	}
	return deleted, nil
}

// PutBlob stores bytes under a sha256 content-addressed path and records metadata.
func (s *Store) PutBlob(root string, data []byte, mime, name string, maxBytes int64) (Blob, error) {
	return s.PutBlobContext(context.Background(), root, data, mime, name, maxBytes)
}

// PutBlobContext stores bytes under a sha256 content-addressed path and observes cancellation before disk and metadata writes.
func (s *Store) PutBlobContext(ctx context.Context, root string, data []byte, mime, name string, maxBytes int64) (Blob, error) {
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return Blob{}, fmt.Errorf("server store: blob is %d bytes, exceeds limit %d", len(data), maxBytes)
	}
	if err := ctx.Err(); err != nil {
		return Blob{}, fmt.Errorf("server store: blob write canceled: %w", err)
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
	defer s.observeDB("PutBlob", time.Now())
	path, err := blobPath(root, hash)
	if err != nil {
		return Blob{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Blob{}, fmt.Errorf("server store: blob mkdir: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Blob{}, fmt.Errorf("server store: blob write canceled: %w", err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return Blob{}, fmt.Errorf("server store: blob write: %w", err)
		}
	} else if err != nil {
		return Blob{}, fmt.Errorf("server store: blob stat: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Blob{}, fmt.Errorf("server store: blob write canceled: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
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
	return s.ReadBlobContext(context.Background(), root, hash)
}

// ReadBlobContext reads content-addressed blob bytes from disk and observes cancellation around disk I/O.
func (s *Store) ReadBlobContext(ctx context.Context, root, hash string) ([]byte, error) {
	path, err := blobPath(root, hash)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("server store: blob read canceled: %w", err)
	}
	if _, ok, err := s.GetBlob(hash); err != nil {
		return nil, err
	} else if !ok {
		return nil, os.ErrNotExist
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("server store: blob read canceled: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("server store: blob read: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("server store: blob read canceled: %w", err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("server store: blob hash mismatch")
	}
	return data, nil
}

// GetBlob returns stored blob metadata.
func (s *Store) GetBlob(hash string) (Blob, bool, error) {
	defer s.observeDB("GetBlob", time.Now())
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
	defer s.observeDB("LinkWorkspaceBlob", time.Now())
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO workspace_blobs(workspace_id, hash) VALUES(?, ?)`, workspaceID, hash); err != nil {
		return fmt.Errorf("server store: link workspace blob: %w", err)
	}
	return nil
}

// UserWorkspaceBlob reports whether a user's workspace is linked to a blob.
func (s *Store) UserWorkspaceBlob(userID, workspaceID, hash string) (bool, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(hash) == "" {
		return false, fmt.Errorf("server store: user id, workspace id, and blob hash are required")
	}
	defer s.observeDB("UserWorkspaceBlob", time.Now())
	var got string
	err := s.db.QueryRow(`
SELECT wb.hash
FROM workspace_blobs wb
JOIN workspaces w ON w.id = wb.workspace_id
WHERE w.user_id = ? AND wb.workspace_id = ? AND wb.hash = ?`, userID, workspaceID, hash).Scan(&got)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("server store: user workspace blob: %w", err)
	}
	return true, nil
}

// UserBlobLinked reports whether any workspace owned by userID already references hash.
func (s *Store) UserBlobLinked(userID, hash string) (bool, error) {
	if strings.TrimSpace(userID) == "" || !validBlobHash(hash) {
		return false, fmt.Errorf("server store: user id and blob hash are required")
	}
	defer s.observeDB("UserBlobLinked", time.Now())
	var count int
	err := s.db.QueryRow(`
SELECT COUNT(*)
FROM workspace_blobs wb
JOIN workspaces w ON w.id = wb.workspace_id
WHERE w.user_id = ? AND wb.hash = ?`, userID, hash).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("server store: user blob linked: %w", err)
	}
	return count > 0, nil
}

// UserBlobBytes returns distinct blob bytes currently referenced by a user's workspaces.
func (s *Store) UserBlobBytes(userID string) (int64, error) {
	if strings.TrimSpace(userID) == "" {
		return 0, fmt.Errorf("server store: user id is required")
	}
	defer s.observeDB("UserBlobBytes", time.Now())
	var total int64
	err := s.db.QueryRow(`
SELECT COALESCE(SUM(size), 0)
FROM (
  SELECT DISTINCT b.hash, b.size
  FROM blobs b
  JOIN workspace_blobs wb ON wb.hash = b.hash
  JOIN workspaces w ON w.id = wb.workspace_id
  WHERE w.user_id = ?
)`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("server store: user blob bytes: %w", err)
	}
	return total, nil
}

// WorkspaceBlobs returns blob metadata linked to a workspace.
func (s *Store) WorkspaceBlobs(workspaceID string) ([]Blob, error) {
	defer s.observeDB("WorkspaceBlobs", time.Now())
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
	defer s.observeDB("SweepUnreferencedBlobs", time.Now())
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
		path, err := blobPath(root, blob.Hash)
		if err != nil {
			return 0, err
		}
		_ = os.Remove(path)
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
	defer s.observeDB("PutAIKey", time.Now())
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
	defer s.observeDB("GetAIKey", time.Now())
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

// AddUsage increments a user's daily request and token counters.
func (s *Store) AddUsage(userID string, day time.Time, requests, tokens int64) (Usage, error) {
	if strings.TrimSpace(userID) == "" {
		return Usage{}, fmt.Errorf("server store: user id is required")
	}
	if requests < 0 || tokens < 0 {
		return Usage{}, fmt.Errorf("server store: usage increments must be non-negative")
	}
	key := usageDay(day)
	defer s.observeDB("AddUsage", time.Now())
	if _, err := s.db.Exec(`
INSERT INTO usage(user_id, day, requests, tokens)
VALUES(?, ?, ?, ?)
ON CONFLICT(user_id, day) DO UPDATE SET
  requests = requests + excluded.requests,
  tokens = tokens + excluded.tokens`,
		userID, key, requests, tokens); err != nil {
		return Usage{}, fmt.Errorf("server store: add usage: %w", err)
	}
	usage, ok, err := s.GetUsage(userID, day)
	if err != nil {
		return Usage{}, err
	}
	if !ok {
		return Usage{}, fmt.Errorf("server store: usage row missing after increment")
	}
	return usage, nil
}

// GetUsage returns a user's usage for the UTC day.
func (s *Store) GetUsage(userID string, day time.Time) (Usage, bool, error) {
	defer s.observeDB("GetUsage", time.Now())
	key := usageDay(day)
	var usage Usage
	err := s.db.QueryRow(`SELECT user_id, day, requests, tokens FROM usage WHERE user_id = ? AND day = ?`, userID, key).
		Scan(&usage.UserID, &usage.Day, &usage.Requests, &usage.Tokens)
	if errors.Is(err, sql.ErrNoRows) {
		return Usage{}, false, nil
	}
	if err != nil {
		return Usage{}, false, fmt.Errorf("server store: get usage: %w", err)
	}
	return usage, true, nil
}

// PutSubscription upserts the current Stripe subscription state for a user.
func (s *Store) PutSubscription(sub Subscription) error {
	if strings.TrimSpace(sub.UserID) == "" || strings.TrimSpace(sub.StripeCustomer) == "" ||
		strings.TrimSpace(sub.StripeSubscription) == "" || strings.TrimSpace(sub.Status) == "" ||
		strings.TrimSpace(sub.Plan) == "" {
		return fmt.Errorf("server store: subscription user, customer, subscription, status, and plan are required")
	}
	if sub.UpdatedAt.IsZero() {
		sub.UpdatedAt = time.Now().UTC()
	}
	defer s.observeDB("PutSubscription", time.Now())
	_, err := s.db.Exec(`
INSERT INTO subscriptions(user_id, stripe_customer, stripe_subscription, status, plan, current_period_end, trial_end, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
  stripe_customer = excluded.stripe_customer,
  stripe_subscription = excluded.stripe_subscription,
  status = excluded.status,
  plan = excluded.plan,
  current_period_end = excluded.current_period_end,
  trial_end = excluded.trial_end,
  updated_at = excluded.updated_at`,
		strings.TrimSpace(sub.UserID),
		strings.TrimSpace(sub.StripeCustomer),
		strings.TrimSpace(sub.StripeSubscription),
		strings.TrimSpace(sub.Status),
		strings.TrimSpace(sub.Plan),
		formatOptionalTime(sub.CurrentPeriodEnd),
		formatOptionalTime(sub.TrialEnd),
		formatTime(sub.UpdatedAt))
	if err != nil {
		return fmt.Errorf("server store: put subscription: %w", err)
	}
	return nil
}

// GetSubscription returns the current subscription row for a user.
func (s *Store) GetSubscription(userID string) (Subscription, bool, error) {
	defer s.observeDB("GetSubscription", time.Now())
	return scanSubscription(s.db.QueryRow(`
SELECT user_id, stripe_customer, stripe_subscription, status, plan, current_period_end, trial_end, updated_at
FROM subscriptions
WHERE user_id = ?`, strings.TrimSpace(userID)))
}

// GetSubscriptionByStripeID returns a row by Stripe subscription id for webhook updates.
func (s *Store) GetSubscriptionByStripeID(stripeSubscription string) (Subscription, bool, error) {
	defer s.observeDB("GetSubscriptionByStripeID", time.Now())
	return scanSubscription(s.db.QueryRow(`
SELECT user_id, stripe_customer, stripe_subscription, status, plan, current_period_end, trial_end, updated_at
FROM subscriptions
WHERE stripe_subscription = ?`, strings.TrimSpace(stripeSubscription)))
}

// UsageWithinLimit reports whether the user has not exceeded the supplied daily limits.
func (s *Store) UsageWithinLimit(userID string, day time.Time, maxRequests, maxTokens int64) (bool, error) {
	if maxRequests < 0 || maxTokens < 0 {
		return false, fmt.Errorf("server store: usage limits must be non-negative")
	}
	usage, ok, err := s.GetUsage(userID, day)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	return usage.Requests <= maxRequests && usage.Tokens <= maxTokens, nil
}

func (s *Store) exportSnapshots(userID string) ([]Snapshot, error) {
	defer s.observeDB("ExportSnapshots", time.Now())
	rows, err := s.db.Query(`
SELECT s.workspace_id, s.dataset_json, s.version, s.updated_at
FROM snapshots s
JOIN workspaces w ON w.id = s.workspace_id
WHERE w.user_id = ?
ORDER BY s.workspace_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("server store: export snapshots: %w", err)
	}
	defer rows.Close()
	return scanSnapshots(rows, "export snapshots")
}

func (s *Store) exportBlobs(userID string) ([]Blob, error) {
	defer s.observeDB("ExportBlobs", time.Now())
	rows, err := s.db.Query(`
SELECT DISTINCT b.hash, b.size, b.mime, b.created_at
FROM blobs b
JOIN workspace_blobs wb ON wb.hash = b.hash
JOIN workspaces w ON w.id = wb.workspace_id
WHERE w.user_id = ?
ORDER BY b.hash`, userID)
	if err != nil {
		return nil, fmt.Errorf("server store: export blobs: %w", err)
	}
	defer rows.Close()
	return scanBlobs(rows, "export blobs")
}

func (s *Store) exportUsage(userID string) ([]Usage, error) {
	defer s.observeDB("ExportUsage", time.Now())
	rows, err := s.db.Query(`SELECT user_id, day, requests, tokens FROM usage WHERE user_id = ? ORDER BY day`, userID)
	if err != nil {
		return nil, fmt.Errorf("server store: export usage: %w", err)
	}
	defer rows.Close()
	var out []Usage
	for rows.Next() {
		var usage Usage
		if err := rows.Scan(&usage.UserID, &usage.Day, &usage.Requests, &usage.Tokens); err != nil {
			return nil, fmt.Errorf("server store: scan export usage: %w", err)
		}
		out = append(out, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: export usage rows: %w", err)
	}
	return out, nil
}

func (s *Store) exportAIKeyProviders(userID string) ([]string, error) {
	defer s.observeDB("ExportAIKeyProviders", time.Now())
	rows, err := s.db.Query(`SELECT provider FROM ai_keys WHERE user_id = ? ORDER BY provider`, userID)
	if err != nil {
		return nil, fmt.Errorf("server store: export ai key providers: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, fmt.Errorf("server store: scan export ai provider: %w", err)
		}
		out = append(out, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: export ai provider rows: %w", err)
	}
	return out, nil
}

func (s *Store) exportAuditEvents(userID string) ([]AuditEvent, error) {
	defer s.observeDB("ExportAuditEvents", time.Now())
	rows, err := s.db.Query(`
SELECT id, timestamp, actor_id, action, target_type, target_id, ip, request_id, previous_hash, hash
FROM audit_events
WHERE actor_id = ?
ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("server store: export audit events: %w", err)
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: export audit event rows: %w", err)
	}
	return out, nil
}

func (s *Store) exportRefreshSessionCount(userID string) (int, error) {
	defer s.observeDB("ExportRefreshSessionCount", time.Now())
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ?`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("server store: export refresh session count: %w", err)
	}
	return count, nil
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

func scanSnapshots(rows *sql.Rows, label string) ([]Snapshot, error) {
	var out []Snapshot
	for rows.Next() {
		snapshot, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: %s rows: %w", label, err)
	}
	return out, nil
}

func scanBlobs(rows *sql.Rows, label string) ([]Blob, error) {
	var out []Blob
	for rows.Next() {
		blob, err := scanBlob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, blob)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("server store: %s rows: %w", label, err)
	}
	return out, nil
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

func usageDay(t time.Time) string { return t.UTC().Format("2006-01-02") }

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatTime(t)
}

func parseOptionalTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	return parseTime(s)
}

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

func blobPath(root, hash string) (string, error) {
	if !validBlobHash(hash) {
		return "", fmt.Errorf("server store: invalid blob hash")
	}
	path := filepath.Join(root, hash[:2], hash[2:4], hash)
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("server store: blob path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("server store: blob path escapes root")
	}
	return path, nil
}
