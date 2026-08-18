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

// Merging two workspaces (TODOS.md C695).
//
// Replace is the right operation when one copy is simply the correct one.
// Merge is the right one when both copies contain real work — which is the
// situation people actually end up in, because the duplicate account is usually
// created partway through a week and both sides get used before anybody
// notices. Without it, the only offered repair asks the user to decide which
// half of their records to throw away.

// PreviewMerge computes what a merge would actually do, by performing it and
// discarding the result.
//
// It does not estimate. A preview that guessed at the counts would be
// describing a different operation than the one about to run, and the whole
// point of previewing a merge is that nobody can hold two households' records
// in their head well enough to predict the overlap.
func (s *Store) PreviewMerge(sourceUserID, sourceWorkspaceID, targetUserID, targetWorkspaceID string, policy datasetmerge.Policy) (MigrationPreview, error) {
	out, target, source, err := s.migrationEnds(MigrateMerge, sourceUserID, sourceWorkspaceID, targetUserID, targetWorkspaceID)
	if err != nil || out.Blocked {
		return out, err
	}
	if len(source) == 0 {
		return blockPreview(out, "the source workspace has no snapshot — there is nothing to merge in"), nil
	}
	if len(target) == 0 {
		// Merging into an empty target is a copy, and saying so is more useful
		// than reporting a merge that turns out to add everything.
		out.Warnings = append(out.Warnings, "the target workspace is empty — this merge copies the source's records in wholesale")
		target = []byte("{}")
	}
	_, report, err := datasetmerge.Merge(target, source, policy)
	if err != nil {
		return blockPreview(out, err.Error()), nil
	}
	out.Merge = &report
	if report.Conflicts > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"%d record(s) exist on both sides with different content; the %q policy decides each one",
			report.Conflicts, string(report.Policy)))
	}
	if len(report.KeptFromTarget) > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"settings and app state are NOT merged — the target keeps its own (%s)", strings.Join(report.KeptFromTarget, ", ")))
	}
	for _, name := range report.UnmergeableCollections {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"%q could not be matched record by record, so the target's copy is kept whole", name))
	}
	return out, nil
}

// MergeWorkspaceSnapshot merges the source's records into the target's snapshot.
//
// It carries the same three guarantees as a replace, for the same reasons: the
// previous contents are archived in the SAME transaction that overwrites them,
// the workspace version moves with the data so the next client pull actually
// sees it, and the attachment links come across so the merged dataset's
// references stay readable.
func (s *Store) MergeWorkspaceSnapshot(sourceUserID, sourceWorkspaceID, targetUserID, targetWorkspaceID string, policy datasetmerge.Policy, now time.Time, requireLocked bool) (MigrationResult, datasetmerge.Report, error) {
	var report datasetmerge.Report
	if s == nil || s.db == nil {
		return MigrationResult{}, report, fmt.Errorf("server store: not configured")
	}
	if requireLocked {
		if err := s.requireBothSuspended(sourceUserID, targetUserID); err != nil {
			return MigrationResult{}, report, err
		}
	}
	source, ok, err := s.GetSnapshotForUser(sourceUserID, sourceWorkspaceID)
	if err != nil {
		return MigrationResult{}, report, err
	}
	if !ok || len(source.Dataset) == 0 {
		return MigrationResult{}, report, fmt.Errorf("server migrate: the source workspace has no snapshot to merge in")
	}
	if _, found, err := s.GetWorkspace(targetUserID, targetWorkspaceID); err != nil {
		return MigrationResult{}, report, err
	} else if !found {
		return MigrationResult{}, report, fmt.Errorf("server migrate: target does not own that workspace")
	}

	defer s.observeDB("MergeWorkspaceSnapshot", time.Now())
	tx, err := s.db.Begin()
	if err != nil {
		return MigrationResult{}, report, fmt.Errorf("server migrate: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		priorDataset []byte
		priorVersion int64
		priorUpdated string
	)
	err = tx.QueryRow("SELECT dataset_json, version, updated_at FROM snapshots WHERE workspace_id = ?", targetWorkspaceID).
		Scan(&priorDataset, &priorVersion, &priorUpdated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		priorDataset = []byte("{}")
	case err != nil:
		return MigrationResult{}, report, fmt.Errorf("server migrate: read target snapshot: %w", err)
	default:
		if _, err := tx.Exec("INSERT INTO snapshot_history(workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?)",
			targetWorkspaceID, priorDataset, priorVersion, priorUpdated); err != nil {
			return MigrationResult{}, report, fmt.Errorf("server migrate: archive target snapshot: %w", err)
		}
	}

	merged, report, err := datasetmerge.Merge(priorDataset, source.Dataset, policy)
	if err != nil {
		return MigrationResult{}, report, fmt.Errorf("server migrate: merge: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO workspace_blobs(workspace_id, hash)
SELECT ?, hash FROM workspace_blobs WHERE workspace_id = ?`,
		targetWorkspaceID, sourceWorkspaceID); err != nil {
		return MigrationResult{}, report, fmt.Errorf("server migrate: carry attachment links: %w", err)
	}

	nextVersion := priorVersion + 1
	stamp := formatTime(now.UTC())
	if _, err := tx.Exec(`INSERT INTO snapshots(workspace_id, dataset_json, version, updated_at) VALUES(?, ?, ?, ?)
ON CONFLICT(workspace_id) DO UPDATE SET dataset_json = excluded.dataset_json, version = excluded.version, updated_at = excluded.updated_at`,
		targetWorkspaceID, merged, nextVersion, stamp); err != nil {
		return MigrationResult{}, report, fmt.Errorf("server migrate: write merged snapshot: %w", err)
	}
	if _, err := tx.Exec("UPDATE workspaces SET version = ?, updated_at = ? WHERE id = ?",
		nextVersion, stamp, targetWorkspaceID); err != nil {
		return MigrationResult{}, report, fmt.Errorf("server migrate: bump target workspace: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MigrationResult{}, report, fmt.Errorf("server migrate: commit: %w", err)
	}
	return MigrationResult{
		Mode: MigrateMerge, WorkspaceID: targetWorkspaceID,
		SourceUserID: sourceUserID, TargetUserID: targetUserID,
		ArchivedVersion: priorVersion, CommittedAt: stamp,
	}, report, nil
}

// migrationEnds resolves and validates both ends of a two-workspace migration,
// returning the shared preview skeleton plus each side's snapshot bytes. Shared
// by the replace and merge previews so the two cannot disagree about what counts
// as a valid pair.
func (s *Store) migrationEnds(mode MigrationMode, sourceUserID, sourceWorkspaceID, targetUserID, targetWorkspaceID string) (MigrationPreview, []byte, []byte, error) {
	out := MigrationPreview{
		Mode:           mode,
		SourceUserID:   strings.TrimSpace(sourceUserID),
		TargetUserID:   strings.TrimSpace(targetUserID),
		WorkspaceID:    strings.TrimSpace(sourceWorkspaceID),
		TargetWorkspac: strings.TrimSpace(targetWorkspaceID),
	}
	if out.SourceUserID == "" || out.TargetUserID == "" || out.WorkspaceID == "" || out.TargetWorkspac == "" {
		return blockPreview(out, "source and target accounts and workspaces are all required"), nil, nil, nil
	}
	source, found, err := s.GetWorkspace(out.SourceUserID, out.WorkspaceID)
	if err != nil {
		return out, nil, nil, err
	}
	if !found {
		return blockPreview(out, ErrMigrationSourceMissing.Error()), nil, nil, nil
	}
	target, found, err := s.GetWorkspace(out.TargetUserID, out.TargetWorkspac)
	if err != nil {
		return out, nil, nil, err
	}
	if !found {
		return blockPreview(out, "target does not own that workspace"), nil, nil, nil
	}
	out.WorkspaceName = source.Name
	out.SourceVersion = source.Version
	out.SourceUpdated = formatTime(source.UpdatedAt)
	out.TargetExists = true
	out.TargetVersion = target.Version
	out.TargetUpdated = formatTime(target.UpdatedAt)

	var sourceBytes, targetBytes []byte
	if snap, ok, err := s.GetSnapshotForUser(out.SourceUserID, out.WorkspaceID); err != nil {
		return out, nil, nil, err
	} else if ok {
		sourceBytes = snap.Dataset
		out.SourceBytes = len(snap.Dataset)
	}
	if snap, ok, err := s.GetSnapshotForUser(out.TargetUserID, out.TargetWorkspac); err != nil {
		return out, nil, nil, err
	} else if ok {
		targetBytes = snap.Dataset
		out.TargetBytes = len(snap.Dataset)
	}
	if out.SourceBlobs, err = s.countWorkspaceBlobs(out.WorkspaceID); err != nil {
		return out, nil, nil, err
	}
	if out.TargetBlobs, err = s.countWorkspaceBlobs(out.TargetWorkspac); err != nil {
		return out, nil, nil, err
	}
	return out, targetBytes, sourceBytes, nil
}
