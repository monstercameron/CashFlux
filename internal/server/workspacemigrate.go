// SPDX-License-Identifier: MIT

package server

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/datasetmerge"
)

// This file moves a workspace from one account to another, and it exists
// because the alternative people reach for is worse (TODOS.md C695, C699).
//
// When a household's data ends up under the wrong account — a second device
// approval minted a new user, and the browser is now signed in as somebody who
// owns none of the workspaces — the intuitive repair is "delete the new account
// and rename the old one", or "export from one, import to the other". Both
// destroy things: the first drops rows through ON DELETE CASCADE, the second
// loses the workspace's identity, so every other device pinned to that
// workspace id starts failing in the same way the first one did.
//
// The actual operation is smaller and safer than either. Snapshots, history and
// blob links are all keyed by workspace_id, never by user_id — so ownership is
// one column on one row. Transferring it moves the data by moving nothing at
// all, which is why C695 states the rule plainly: never implement overwrite as
// account deletion plus rename.

// MigrationMode is what to do when the target account already has a workspace
// in the way.
type MigrationMode string

const (
	// MigrateTransfer repoints an existing workspace at a new owner. Nothing is
	// copied and nothing is deleted.
	MigrateTransfer MigrationMode = "transfer"
	// MigrateReplace overwrites the TARGET workspace's snapshot with the
	// source's, after archiving what was there.
	MigrateReplace MigrationMode = "replace"
	// MigrateMerge unions the two snapshots record by record, keeping what only
	// one side has and deciding same-id disagreements by an explicit policy.
	// Replace is the right operation when one copy is simply the correct one;
	// merge is the right one when both copies contain real work.
	MigrateMerge MigrationMode = "merge"
)

// MigrationPreview is what an operator is shown BEFORE anything happens: what
// exists on each side, and what the commit would do. C699 requires the preview
// because a migration decided from memory is how data lands on the wrong
// account.
type MigrationPreview struct {
	Mode           MigrationMode `json:"mode"`
	SourceUserID   string        `json:"sourceUserId"`
	TargetUserID   string        `json:"targetUserId"`
	WorkspaceID    string        `json:"workspaceId"`
	WorkspaceName  string        `json:"workspaceName"`
	SourceBytes    int           `json:"sourceBytes"`
	SourceVersion  int64         `json:"sourceVersion"`
	SourceUpdated  string        `json:"sourceUpdatedAt,omitempty"`
	SourceBlobs    int           `json:"sourceBlobs"`
	TargetExists   bool          `json:"targetExists"`
	TargetBytes    int           `json:"targetBytes,omitempty"`
	TargetVersion  int64         `json:"targetVersion,omitempty"`
	TargetUpdated  string        `json:"targetUpdatedAt,omitempty"`
	TargetBlobs    int           `json:"targetBlobs,omitempty"`
	TargetWorkspac string        `json:"targetWorkspaceId,omitempty"`
	// Warnings are the things an operator should read before confirming. They
	// are stated rather than enforced where the operation is legitimate but
	// lossy — refusing outright would leave a real situation unfixable.
	Warnings []string `json:"warnings,omitempty"`
	// Blocked is set when the migration cannot proceed at all; Reason says why.
	Blocked bool   `json:"blocked"`
	Reason  string `json:"reason,omitempty"`
	// Merge is the record-by-record outcome a merge WOULD have, computed by
	// actually performing it against the two snapshots and throwing the result
	// away. A merge preview that estimated instead of computing would be
	// describing a different operation than the one about to run.
	Merge *datasetmerge.Report `json:"merge,omitempty"`
}

// MigrationResult records what a committed migration actually did.
type MigrationResult struct {
	Mode         MigrationMode `json:"mode"`
	WorkspaceID  string        `json:"workspaceId"`
	SourceUserID string        `json:"sourceUserId"`
	TargetUserID string        `json:"targetUserId"`
	// ArchivedVersion is the snapshot_history version the previous target
	// contents were written to, so a rollback has a specific thing to name.
	ArchivedVersion int64  `json:"archivedVersion,omitempty"`
	CommittedAt     string `json:"committedAt"`
}

var (
	// ErrMigrationSourceMissing means the workspace is not owned by the source.
	ErrMigrationSourceMissing = errors.New("server migrate: source does not own that workspace")
	// ErrMigrationTargetExists is returned when the target account already owns a
	// workspace with the id being transferred. Only reachable since workspace ids
	// became per-account: two households can now legitimately both hold
	// "default", and folding one into the other is a merge, not a transfer.
	ErrMigrationTargetExists = errors.New("server migrate: the target account already owns a workspace with that id")
	// ErrMigrationTargetMissing means the target account does not exist.
	ErrMigrationTargetMissing = errors.New("server migrate: no such target account")
	// ErrMigrationSameAccount means source and target are the same account.
	ErrMigrationSameAccount = errors.New("server migrate: source and target are the same account")
	// ErrMigrationTargetUnlocked means one of the two accounts was not suspended
	// for the operation.
	//
	// C699 asks for the target to be locked. Adversarial review (2026-08-17)
	// showed that is not enough: SyncService.PutWorkspace authorizes ownership
	// with separate reads and then upserts the row with no ownership guard, so a
	// SOURCE device that is still syncing can land its write after the transfer
	// commits, flip user_id back, and overwrite the moved snapshot with stale
	// data — silently, with the migration reported as successful. Both sides are
	// therefore locked, which closes the window rather than narrowing it.
	ErrMigrationTargetUnlocked = errors.New("server migrate: both accounts must be suspended for the duration")
)

// PreviewMigration assembles the before picture without changing anything.
func (s *Store) PreviewMigration(mode MigrationMode, sourceUserID, targetUserID, workspaceID string) (MigrationPreview, error) {
	if s == nil || s.db == nil {
		return MigrationPreview{}, fmt.Errorf("server store: not configured")
	}
	sourceUserID = strings.TrimSpace(sourceUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	workspaceID = strings.TrimSpace(workspaceID)
	out := MigrationPreview{
		Mode: mode, SourceUserID: sourceUserID, TargetUserID: targetUserID, WorkspaceID: workspaceID,
	}
	if sourceUserID == "" || targetUserID == "" || workspaceID == "" {
		return blockPreview(out, "source, target and workspace are all required"), nil
	}
	if sourceUserID == targetUserID {
		return blockPreview(out, ErrMigrationSameAccount.Error()), nil
	}
	if _, found, err := s.GetUserByID(targetUserID); err != nil {
		return MigrationPreview{}, err
	} else if !found {
		return blockPreview(out, ErrMigrationTargetMissing.Error()), nil
	}
	source, found, err := s.GetWorkspace(sourceUserID, workspaceID)
	if err != nil {
		return MigrationPreview{}, err
	}
	if !found {
		return blockPreview(out, ErrMigrationSourceMissing.Error()), nil
	}
	out.WorkspaceName = source.Name
	out.SourceVersion = source.Version
	out.SourceUpdated = formatTime(source.UpdatedAt)
	if snap, ok, err := s.GetSnapshotForUser(sourceUserID, workspaceID); err != nil {
		return MigrationPreview{}, err
	} else if ok {
		out.SourceBytes = len(snap.Dataset)
	}
	if n, err := s.countWorkspaceBlobs(sourceUserID, workspaceID); err != nil {
		return MigrationPreview{}, err
	} else {
		out.SourceBlobs = n
	}

	switch mode {
	case MigrateTransfer:
		// A transfer collides only if the target already owns this exact id,
		// which cannot happen — ownership is one row — so the only real
		// question is whether the target has a same-NAMED workspace, which is
		// confusing but harmless.
		targets, err := s.ListWorkspaces(targetUserID, false)
		if err != nil {
			return MigrationPreview{}, err
		}
		for _, t := range targets {
			if strings.EqualFold(strings.TrimSpace(t.Name), strings.TrimSpace(source.Name)) {
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"the target already has a workspace named %q (%s) — after the transfer it will have two", t.Name, t.ID))
			}
		}
		if out.SourceBytes == 0 {
			out.Warnings = append(out.Warnings, "the source workspace has no snapshot yet — this transfers an empty workspace")
		}
	case MigrateReplace:
		return MigrationPreview{}, fmt.Errorf("server migrate: replace requires a target workspace; use PreviewReplace")
	default:
		return blockPreview(out, fmt.Sprintf("unknown migration mode %q", mode)), nil
	}
	return out, nil
}

// PreviewReplace assembles the before picture for overwriting one workspace's
// contents with another's. The two workspaces keep their own identities: only
// the dataset moves, so every device pinned to the target's id keeps working.
func (s *Store) PreviewReplace(sourceUserID, sourceWorkspaceID, targetUserID, targetWorkspaceID string) (MigrationPreview, error) {
	if s == nil || s.db == nil {
		return MigrationPreview{}, fmt.Errorf("server store: not configured")
	}
	out := MigrationPreview{
		Mode:           MigrateReplace,
		SourceUserID:   strings.TrimSpace(sourceUserID),
		TargetUserID:   strings.TrimSpace(targetUserID),
		WorkspaceID:    strings.TrimSpace(sourceWorkspaceID),
		TargetWorkspac: strings.TrimSpace(targetWorkspaceID),
	}
	if out.SourceUserID == "" || out.TargetUserID == "" || out.WorkspaceID == "" || out.TargetWorkspac == "" {
		return blockPreview(out, "source and target accounts and workspaces are all required"), nil
	}
	source, found, err := s.GetWorkspace(out.SourceUserID, out.WorkspaceID)
	if err != nil {
		return MigrationPreview{}, err
	}
	if !found {
		return blockPreview(out, ErrMigrationSourceMissing.Error()), nil
	}
	target, found, err := s.GetWorkspace(out.TargetUserID, out.TargetWorkspac)
	if err != nil {
		return MigrationPreview{}, err
	}
	if !found {
		return blockPreview(out, "target does not own that workspace"), nil
	}
	out.WorkspaceName = source.Name
	out.SourceVersion = source.Version
	out.SourceUpdated = formatTime(source.UpdatedAt)
	out.TargetExists = true
	out.TargetVersion = target.Version
	out.TargetUpdated = formatTime(target.UpdatedAt)
	if snap, ok, err := s.GetSnapshotForUser(out.SourceUserID, out.WorkspaceID); err != nil {
		return MigrationPreview{}, err
	} else if ok {
		out.SourceBytes = len(snap.Dataset)
	}
	if snap, ok, err := s.GetSnapshotForUser(out.TargetUserID, out.TargetWorkspac); err != nil {
		return MigrationPreview{}, err
	} else if ok {
		out.TargetBytes = len(snap.Dataset)
	}
	if out.SourceBlobs, err = s.countWorkspaceBlobs(out.SourceUserID, out.WorkspaceID); err != nil {
		return MigrationPreview{}, err
	}
	if out.TargetBlobs, err = s.countWorkspaceBlobs(out.TargetUserID, out.TargetWorkspac); err != nil {
		return MigrationPreview{}, err
	}
	if out.SourceBytes == 0 {
		return blockPreview(out, "the source workspace has no snapshot — replacing with nothing would erase the target"), nil
	}
	if out.TargetBytes > out.SourceBytes {
		// Stated, not blocked. A smaller replacement is often exactly right
		// (a cleaned-up dataset), and refusing it would make a legitimate
		// repair impossible. But an operator should see it before confirming.
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"the replacement is smaller than what it overwrites (%d bytes replacing %d) — confirm this is the copy you mean to keep",
			out.SourceBytes, out.TargetBytes))
	}
	if out.TargetBlobs > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"%d attachment(s) stay linked to the target workspace; replacing the dataset does not move or delete them", out.TargetBlobs))
	}
	return out, nil
}

// requireBothSuspended enforces that neither account can be written to while its
// data changes hands. Suspension is what stops an in-flight client sync
// (requireWriter rejects a suspended caller), and an unsuspended SOURCE is just
// as dangerous as an unsuspended target — see ErrMigrationTargetUnlocked.
func (s *Store) requireBothSuspended(sourceUserID, targetUserID string) error {
	for _, id := range []string{sourceUserID, targetUserID} {
		locked, err := s.IsUserSuspended(id)
		if err != nil {
			return err
		}
		if !locked {
			return ErrMigrationTargetUnlocked
		}
	}
	return nil
}

func blockPreview(p MigrationPreview, reason string) MigrationPreview {
	p.Blocked = true
	p.Reason = reason
	return p
}

// countWorkspaceBlobs counts a workspace's attachment links.
func (s *Store) countWorkspaceBlobs(userID, workspaceID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM workspace_blobs WHERE user_id = ? AND workspace_id = ?`, userID, workspaceID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("server store: count workspace blobs: %w", err)
	}
	return n, nil
}

// TransferWorkspace moves ownership of one workspace to another account, in a
// single transaction.
//
// Nothing is copied — only re-owned — so there is no half-copied state to
// recover from, and every device already pinned to this workspace id keeps
// working under the new owner's credentials, which is the entire objective.
//
// It used to be a single-column update, because snapshots, snapshot_history and
// workspace_blobs were keyed by workspace_id alone and so followed the workspace
// wherever it went. They now carry their owner too (workspace ids are minted by
// clients and collide across accounts), so the transfer has to move them
// explicitly — in the same transaction, or a failure halfway would leave a
// workspace owned by one account and its data by another.
//
// A target that already owns a workspace with this id is a genuine conflict now
// rather than an impossibility, and is refused: merging two households because
// they happened to pick the same id is not a transfer.
//
// requireTargetLocked implements C699's rule that the target is suspended for
// the duration. A device writing to the target while ownership changes would
// race the transfer.
func (s *Store) TransferWorkspace(sourceUserID, targetUserID, workspaceID string, now time.Time, requireTargetLocked bool) (MigrationResult, error) {
	if s == nil || s.db == nil {
		return MigrationResult{}, fmt.Errorf("server store: not configured")
	}
	sourceUserID = strings.TrimSpace(sourceUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	workspaceID = strings.TrimSpace(workspaceID)
	if sourceUserID == "" || targetUserID == "" || workspaceID == "" {
		return MigrationResult{}, fmt.Errorf("server migrate: source, target and workspace are required")
	}
	if sourceUserID == targetUserID {
		return MigrationResult{}, ErrMigrationSameAccount
	}
	if _, found, err := s.GetUserByID(targetUserID); err != nil {
		return MigrationResult{}, err
	} else if !found {
		return MigrationResult{}, ErrMigrationTargetMissing
	}
	if requireTargetLocked {
		if err := s.requireBothSuspended(sourceUserID, targetUserID); err != nil {
			return MigrationResult{}, err
		}
	}
	defer s.observeDB("TransferWorkspace", time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Conditional on the CURRENT owner, so two operators racing the same
	// migration cannot both succeed and so a stale console view cannot move a
	// workspace that has already moved.
	// Source ownership is checked FIRST. Reporting a target clash to somebody who
	// named the wrong source would answer a question they did not ask, and hide
	// the one thing actually wrong with their request.
	var owned int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE user_id = ? AND id = ?`,
		sourceUserID, workspaceID).Scan(&owned); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: check source: %w", err)
	}
	if owned == 0 {
		return MigrationResult{}, ErrMigrationSourceMissing
	}
	var clash int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM workspaces WHERE user_id = ? AND id = ?`,
		targetUserID, workspaceID).Scan(&clash); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: check target: %w", err)
	}
	if clash > 0 {
		return MigrationResult{}, ErrMigrationTargetExists
	}
	res, err := tx.Exec(`UPDATE workspaces SET user_id = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		targetUserID, formatTime(now.UTC()), workspaceID, sourceUserID)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: transfer: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: transfer: %w", err)
	}
	if affected == 0 {
		return MigrationResult{}, ErrMigrationSourceMissing
	}
	// The workspace's data has to follow it. Same transaction as the line above,
	// so ownership can never end up split between two accounts.
	for _, table := range []string{"snapshots", "snapshot_history", "workspace_blobs"} {
		if _, err := tx.Exec(
			`UPDATE `+table+` SET user_id = ? WHERE user_id = ? AND workspace_id = ?`,
			targetUserID, sourceUserID, workspaceID); err != nil {
			return MigrationResult{}, fmt.Errorf("server migrate: transfer %s: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: commit: %w", err)
	}
	return MigrationResult{
		Mode: MigrateTransfer, WorkspaceID: workspaceID,
		SourceUserID: sourceUserID, TargetUserID: targetUserID,
		CommittedAt: formatTime(now.UTC()),
	}, nil
}

// ReplaceWorkspaceSnapshot overwrites the target workspace's dataset with the
// source's, archiving what was there first.
//
// The archive is not optional and not best-effort: it is written in the same
// transaction as the overwrite, so either the previous contents are recoverable
// or the overwrite did not happen. An "are you sure" dialog is not a backup.
func (s *Store) ReplaceWorkspaceSnapshot(sourceUserID, sourceWorkspaceID, targetUserID, targetWorkspaceID string, now time.Time, requireTargetLocked bool) (MigrationResult, error) {
	if s == nil || s.db == nil {
		return MigrationResult{}, fmt.Errorf("server store: not configured")
	}
	sourceUserID = strings.TrimSpace(sourceUserID)
	sourceWorkspaceID = strings.TrimSpace(sourceWorkspaceID)
	targetUserID = strings.TrimSpace(targetUserID)
	targetWorkspaceID = strings.TrimSpace(targetWorkspaceID)
	if sourceUserID == "" || sourceWorkspaceID == "" || targetUserID == "" || targetWorkspaceID == "" {
		return MigrationResult{}, fmt.Errorf("server migrate: source and target accounts and workspaces are required")
	}
	if requireTargetLocked {
		if err := s.requireBothSuspended(sourceUserID, targetUserID); err != nil {
			return MigrationResult{}, err
		}
	}
	source, ok, err := s.GetSnapshotForUser(sourceUserID, sourceWorkspaceID)
	if err != nil {
		return MigrationResult{}, err
	}
	if !ok || len(source.Dataset) == 0 {
		return MigrationResult{}, fmt.Errorf("server migrate: the source workspace has no snapshot to copy")
	}
	if _, found, err := s.GetWorkspace(targetUserID, targetWorkspaceID); err != nil {
		return MigrationResult{}, err
	} else if !found {
		return MigrationResult{}, fmt.Errorf("server migrate: target does not own that workspace")
	}

	defer s.observeDB("ReplaceWorkspaceSnapshot", time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		priorDataset []byte
		priorVersion int64
		priorUpdated string
	)
	err = tx.QueryRow(`SELECT dataset_json, version, updated_at FROM snapshots WHERE user_id = ? AND workspace_id = ?`,
		targetUserID, targetWorkspaceID).
		Scan(&priorDataset, &priorVersion, &priorUpdated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Nothing to archive — the target has never been written.
	case err != nil:
		return MigrationResult{}, fmt.Errorf("server migrate: read target snapshot: %w", err)
	default:
		if _, err := tx.Exec(`INSERT INTO snapshot_history(user_id, workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?, ?)`,
			targetUserID, targetWorkspaceID, priorDataset, priorVersion, priorUpdated); err != nil {
			return MigrationResult{}, fmt.Errorf("server migrate: archive target snapshot: %w", err)
		}
	}

	// The dataset carries blob HASHES inside it (domain.Artifact.BlobRef), but
	// authorization to fetch those bytes comes from a workspace_blobs row for the
	// TARGET workspace id. Copying the dataset without the links leaves every
	// attachment referenced-but-unreadable for the new owner, and — once the
	// source is cleaned up and the sweeper runs — deletes the bytes outright,
	// long after the migration and with nothing pointing back at it. Found by
	// adversarial review, 2026-08-17.
	//
	// The links are copied, not moved: the source keeps its own, so this cannot
	// break the account the data came from either. Blobs are content-addressed
	// and shared by hash, so two links to one blob is the normal state.
	if _, err := tx.Exec(`INSERT OR IGNORE INTO workspace_blobs(user_id, workspace_id, hash)
SELECT ?, ?, hash FROM workspace_blobs WHERE user_id = ? AND workspace_id = ?`,
		targetUserID, targetWorkspaceID, sourceUserID, sourceWorkspaceID); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: carry attachment links: %w", err)
	}

	nextVersion := priorVersion + 1
	stamp := formatTime(now.UTC())
	if _, err := tx.Exec(`INSERT INTO snapshots(user_id, workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?, ?)
ON CONFLICT(user_id, workspace_id) DO UPDATE SET dataset_json = excluded.dataset_json, version = excluded.version, updated_at = excluded.updated_at`,
		targetUserID, targetWorkspaceID, source.Dataset, nextVersion, stamp); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: write target snapshot: %w", err)
	}
	// The workspace row's version must move with its snapshot, or the next
	// client pull compares against a stale version and decides it is already
	// up to date — the overwrite would be invisible to the very device it was
	// performed for.
	if _, err := tx.Exec(`UPDATE workspaces SET version = ?, updated_at = ? WHERE user_id = ? AND id = ?`,
		nextVersion, stamp, targetUserID, targetWorkspaceID); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: bump target workspace: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: commit: %w", err)
	}
	return MigrationResult{
		Mode: MigrateReplace, WorkspaceID: targetWorkspaceID,
		SourceUserID: sourceUserID, TargetUserID: targetUserID,
		ArchivedVersion: priorVersion, CommittedAt: stamp,
	}, nil
}

// RollbackWorkspaceSnapshot restores a workspace to an archived version — the
// undo half C695 and C699 both require.
//
// It archives the CURRENT contents on the way back, so a rollback is itself
// reversible. An undo that destroys the thing it undid is a second incident.
func (s *Store) RollbackWorkspaceSnapshot(userID, workspaceID string, version int64, now time.Time) (MigrationResult, error) {
	if s == nil || s.db == nil {
		return MigrationResult{}, fmt.Errorf("server store: not configured")
	}
	userID = strings.TrimSpace(userID)
	workspaceID = strings.TrimSpace(workspaceID)
	if userID == "" || workspaceID == "" {
		return MigrationResult{}, fmt.Errorf("server migrate: account and workspace are required")
	}
	if _, found, err := s.GetWorkspace(userID, workspaceID); err != nil {
		return MigrationResult{}, err
	} else if !found {
		return MigrationResult{}, fmt.Errorf("server migrate: that account does not own that workspace")
	}
	defer s.observeDB("RollbackWorkspaceSnapshot", time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var restore []byte
	err = tx.QueryRow(`SELECT dataset_json FROM snapshot_history WHERE user_id = ? AND workspace_id = ? AND version = ?
ORDER BY id DESC LIMIT 1`, userID, workspaceID, version).Scan(&restore)
	if errors.Is(err, sql.ErrNoRows) {
		return MigrationResult{}, fmt.Errorf("server migrate: no archived version %d for that workspace", version)
	}
	if err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: read archive: %w", err)
	}

	var (
		currentDataset []byte
		currentVersion int64
		currentUpdated string
	)
	err = tx.QueryRow(`SELECT dataset_json, version, updated_at FROM snapshots WHERE user_id = ? AND workspace_id = ?`, userID, workspaceID).
		Scan(&currentDataset, &currentVersion, &currentUpdated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return MigrationResult{}, fmt.Errorf("server migrate: read current snapshot: %w", err)
	}
	if err == nil {
		if _, err := tx.Exec(`INSERT INTO snapshot_history(user_id, workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?, ?)`,
			userID, workspaceID, currentDataset, currentVersion, currentUpdated); err != nil {
			return MigrationResult{}, fmt.Errorf("server migrate: archive before rollback: %w", err)
		}
	}

	nextVersion := currentVersion + 1
	stamp := formatTime(now.UTC())
	if _, err := tx.Exec(`INSERT INTO snapshots(user_id, workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?, ?)
ON CONFLICT(user_id, workspace_id) DO UPDATE SET dataset_json = excluded.dataset_json, version = excluded.version, updated_at = excluded.updated_at`,
		userID, workspaceID, restore, nextVersion, stamp); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: restore snapshot: %w", err)
	}
	if _, err := tx.Exec(`UPDATE workspaces SET version = ?, updated_at = ? WHERE user_id = ? AND id = ?`,
		nextVersion, stamp, userID, workspaceID); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: bump workspace: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MigrationResult{}, fmt.Errorf("server migrate: commit: %w", err)
	}
	return MigrationResult{
		Mode: MigrateReplace, WorkspaceID: workspaceID,
		SourceUserID: userID, TargetUserID: userID,
		ArchivedVersion: currentVersion, CommittedAt: stamp,
	}, nil
}
