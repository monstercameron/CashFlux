// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The operator-facing half of the migration workflow (TODOS.md C699).
//
// Every route here is preview-first and audited. The preview is a separate call
// rather than a field on the commit response because the operator has to be able
// to READ the consequences and then decide — a preview returned alongside the
// thing it was previewing would be a receipt, not a decision point.

// adminMigratePreviewRequest asks what a migration would do.
type adminMigratePreviewRequest struct {
	Mode              string `json:"mode"`
	SourceUserID      string `json:"sourceUserId"`
	TargetUserID      string `json:"targetUserId"`
	WorkspaceID       string `json:"workspaceId"`
	TargetWorkspaceID string `json:"targetWorkspaceId,omitempty"`
}

// adminMigrateCommitRequest performs one. Confirm must carry the workspace id
// the operator believes they are moving: a mistyped or stale id fails the check
// instead of migrating something else. It is deliberately not a boolean —
// "confirm: true" is a checkbox, and a checkbox does not prove the operator read
// which workspace they were looking at.
type adminMigrateCommitRequest struct {
	adminMigratePreviewRequest
	Confirm string `json:"confirm"`
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(dst); err != nil {
		writeErrorJSON(w, ErrorReasonInvalidArgument, "invalid request body")
		return false
	}
	return true
}

// handleAdminMigratePreview serves POST /v1/admin/migrations/preview.
func handleAdminMigratePreview(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := adminAuthorize(cfg, store, w, r, "admin.migration.preview", "workspace"); !ok {
			return
		}
		var req adminMigratePreviewRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		preview, err := migrationPreview(store, req)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "preview failed")
			return
		}
		writeJSON(w, preview)
	}
}

// parseMigrationMode resolves the requested mode, rejecting anything it does not
// recognise.
//
// It used to fall through to a transfer for ANY unrecognised string. A typo or a
// client bug sending "Replace" therefore ran the wrong operation — moving the
// source's whole workspace to the target account instead of overwriting the
// target's data — and the audit line was built from the raw string, so the
// record could name an operation that never happened. Found by adversarial
// review, 2026-08-17. An empty mode still means transfer, which is a stated
// default rather than a guess.
func parseMigrationMode(raw string) (MigrationMode, bool) {
	switch MigrationMode(strings.TrimSpace(raw)) {
	case MigrateReplace:
		return MigrateReplace, true
	case MigrateTransfer, "":
		return MigrateTransfer, true
	}
	return "", false
}

func migrationPreview(store *Store, req adminMigratePreviewRequest) (MigrationPreview, error) {
	mode, ok := parseMigrationMode(req.Mode)
	if !ok {
		return blockPreview(MigrationPreview{Mode: MigrationMode(req.Mode)},
			fmt.Sprintf("%q is not a migration mode; use \"transfer\" or \"replace\"", req.Mode)), nil
	}
	if mode == MigrateReplace {
		return store.PreviewReplace(req.SourceUserID, req.WorkspaceID, req.TargetUserID, req.TargetWorkspaceID)
	}
	return store.PreviewMigration(MigrateTransfer, req.SourceUserID, req.TargetUserID, req.WorkspaceID)
}

// handleAdminMigrateCommit serves POST /v1/admin/migrations/commit.
func handleAdminMigrateCommit(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.migration.commit", "workspace")
		if !ok {
			return
		}
		var req adminMigrateCommitRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		mode, ok := parseMigrationMode(req.Mode)
		if !ok {
			writeErrorJSON(w, ErrorReasonInvalidArgument,
				"unknown migration mode — use \"transfer\" or \"replace\"")
			return
		}
		// The workspace the operator names in `confirm` must be the one being
		// acted on. For a replace that is the TARGET, since the target is what
		// gets overwritten and is therefore what can be lost.
		expected := strings.TrimSpace(req.WorkspaceID)
		if mode == MigrateReplace {
			expected = strings.TrimSpace(req.TargetWorkspaceID)
		}
		if strings.TrimSpace(req.Confirm) != expected || expected == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition,
				"confirm must repeat the id of the workspace being changed")
			return
		}
		// Re-preview immediately before committing. The console's preview may be
		// minutes old, and the thing it described may have moved since — that is
		// exactly the window in which a second operator resolves the same case.
		preview, err := migrationPreview(store, req.adminMigratePreviewRequest)
		if err != nil {
			writeErrorJSON(w, ErrorReasonInternal, "preview failed")
			return
		}
		if preview.Blocked {
			writeErrorJSON(w, ErrorReasonFailedPrecondition, preview.Reason)
			return
		}

		now := time.Now().UTC()
		var result MigrationResult
		switch mode {
		case MigrateReplace:
			result, err = store.ReplaceWorkspaceSnapshot(
				req.SourceUserID, req.WorkspaceID, req.TargetUserID, req.TargetWorkspaceID, now, true)
		default:
			result, err = store.TransferWorkspace(req.SourceUserID, req.TargetUserID, req.WorkspaceID, now, true)
		}
		if !writeMigrationError(w, err) {
			return
		}
		auditFromRequest(r, store, admin, "admin.migration."+string(mode), "workspace",
			result.WorkspaceID+" "+result.SourceUserID+"->"+result.TargetUserID)
		writeJSON(w, result)
	}
}

// adminRollbackRequest names an archived version to restore.
type adminRollbackRequest struct {
	UserID      string `json:"userId"`
	WorkspaceID string `json:"workspaceId"`
	Version     int64  `json:"version"`
	Confirm     string `json:"confirm"`
}

// handleAdminMigrateRollback serves POST /v1/admin/migrations/rollback.
func handleAdminMigrateRollback(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := adminAuthorize(cfg, store, w, r, "admin.migration.rollback", "workspace")
		if !ok {
			return
		}
		var req adminRollbackRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Confirm) != strings.TrimSpace(req.WorkspaceID) || strings.TrimSpace(req.WorkspaceID) == "" {
			writeErrorJSON(w, ErrorReasonFailedPrecondition,
				"confirm must repeat the id of the workspace being restored")
			return
		}
		result, err := store.RollbackWorkspaceSnapshot(req.UserID, req.WorkspaceID, req.Version, time.Now().UTC())
		if !writeMigrationError(w, err) {
			return
		}
		auditFromRequest(r, store, admin, "admin.migration.rollback", "workspace",
			result.WorkspaceID+"@"+strconv.FormatInt(req.Version, 10))
		writeJSON(w, result)
	}
}

// writeMigrationError maps a migration failure onto a precise reason and reports
// whether the caller may continue. A migration that fails needs to say WHICH
// precondition it failed — "internal error" leaves an operator with a
// half-understood situation and a strong urge to try something destructive.
func writeMigrationError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, ErrMigrationSourceMissing):
		writeErrorJSON(w, ErrorReasonFailedPrecondition, "the source account no longer owns that workspace")
	case errors.Is(err, ErrMigrationTargetMissing):
		writeErrorJSON(w, ErrorReasonNotFound, "no such target account")
	case errors.Is(err, ErrMigrationSameAccount):
		writeErrorJSON(w, ErrorReasonInvalidArgument, "source and target are the same account")
	case errors.Is(err, ErrMigrationTargetUnlocked):
		writeErrorJSON(w, ErrorReasonFailedPrecondition,
			"suspend the target account first — it must be locked while its data changes hands")
	default:
		writeErrorJSON(w, ErrorReasonInternal, "migration failed")
	}
	return false
}
