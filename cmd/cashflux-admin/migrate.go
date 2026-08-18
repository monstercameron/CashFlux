// SPDX-License-Identifier: MIT

//go:build js && wasm

package main

// The migration wizard (TODOS.md C699).
//
// The endpoints behind it are preview-first for a reason, and the UI has to
// preserve that or it throws the guarantee away: an operator who can commit
// without having read a preview is back to migrating from memory, which is how
// data lands on the wrong account. So Commit does not exist until a preview has
// been fetched and shown, it is disabled while the form differs from what was
// previewed, and it requires the operator to retype the id of the workspace
// that is about to change.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// adminCollectionChange mirrors datasetmerge.CollectionChange.
type adminCollectionChange struct {
	Name       string `json:"name"`
	Added      int    `json:"added"`
	Conflicts  int    `json:"conflicts"`
	Identical  int    `json:"identical"`
	TargetOnly int    `json:"targetOnly"`
	Total      int    `json:"total"`
}

// adminMergeReport mirrors datasetmerge.Report.
type adminMergeReport struct {
	Policy                 string                  `json:"policy"`
	Collections            []adminCollectionChange `json:"collections"`
	TotalAdded             int                     `json:"totalAdded"`
	Conflicts              int                     `json:"conflicts"`
	KeptFromTarget         []string                `json:"keptFromTarget"`
	UnmergeableCollections []string                `json:"unmergeableCollections"`
}

// adminMigrationPreview mirrors server.MigrationPreview.
type adminMigrationPreview struct {
	Mode              string            `json:"mode"`
	SourceUserID      string            `json:"sourceUserId"`
	TargetUserID      string            `json:"targetUserId"`
	WorkspaceID       string            `json:"workspaceId"`
	WorkspaceName     string            `json:"workspaceName"`
	SourceBytes       int               `json:"sourceBytes"`
	SourceVersion     int64             `json:"sourceVersion"`
	SourceUpdated     string            `json:"sourceUpdatedAt"`
	SourceBlobs       int               `json:"sourceBlobs"`
	TargetExists      bool              `json:"targetExists"`
	TargetBytes       int               `json:"targetBytes"`
	TargetVersion     int64             `json:"targetVersion"`
	TargetUpdated     string            `json:"targetUpdatedAt"`
	TargetBlobs       int               `json:"targetBlobs"`
	TargetWorkspaceID string            `json:"targetWorkspaceId"`
	Warnings          []string          `json:"warnings"`
	Blocked           bool              `json:"blocked"`
	Reason            string            `json:"reason"`
	Merge             *adminMergeReport `json:"merge"`
}

// adminMigrationResult mirrors server.MigrationResult (+ the merge counts).
type adminMigrationResult struct {
	Mode            string            `json:"mode"`
	WorkspaceID     string            `json:"workspaceId"`
	SourceUserID    string            `json:"sourceUserId"`
	TargetUserID    string            `json:"targetUserId"`
	ArchivedVersion int64             `json:"archivedVersion"`
	CommittedAt     string            `json:"committedAt"`
	Merge           *adminMergeReport `json:"merge"`
}

type migrateForm struct {
	Mode              string `json:"mode"`
	SourceUserID      string `json:"sourceUserId"`
	TargetUserID      string `json:"targetUserId"`
	WorkspaceID       string `json:"workspaceId"`
	TargetWorkspaceID string `json:"targetWorkspaceId,omitempty"`
	Policy            string `json:"policy,omitempty"`
	Confirm           string `json:"confirm,omitempty"`
}

func postMigratePreview(token string, form migrateForm) (*adminMigrationPreview, error) {
	body, _ := json.Marshal(form)
	code, out, err := adminDo(token, "POST", "/v1/admin/migrations/preview", string(body))
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("preview: HTTP %d %s", code, strings.TrimSpace(string(out)))
	}
	var p adminMigrationPreview
	if err := json.Unmarshal(out, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func postMigrateCommit(token string, form migrateForm) (*adminMigrationResult, error) {
	body, _ := json.Marshal(form)
	code, out, err := adminDo(token, "POST", "/v1/admin/migrations/commit", string(body))
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("commit: HTTP %d %s", code, strings.TrimSpace(string(out)))
	}
	var r adminMigrationResult
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func postMigrateRollback(token, userID, workspaceID string, version int64) error {
	body, _ := json.Marshal(map[string]any{
		"userId": userID, "workspaceId": workspaceID, "version": version, "confirm": workspaceID,
	})
	code, out, err := adminDo(token, "POST", "/v1/admin/migrations/rollback", string(body))
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("rollback: HTTP %d %s", code, strings.TrimSpace(string(out)))
	}
	return nil
}

type migrateProps struct {
	token string
	// targetUserID pre-fills the account whose page the wizard was opened from,
	// so the commonest direction (bring another account's data HERE) needs one
	// id typed instead of two.
	targetUserID string
}

// migrateWizard renders the source/target form, the preview, and the commit.
func migrateWizard(p migrateProps) ui.Node {
	open := ui.UseState(false)
	mode := ui.UseState("transfer")
	policy := ui.UseState("prefer-target")
	sourceUser := ui.UseState("")
	sourceWS := ui.UseState("")
	targetWS := ui.UseState("")
	confirm := ui.UseState("")
	preview := ui.UseState[*adminMigrationPreview](nil)
	// previewedFor is the exact form the preview describes. Commit is refused
	// unless the form still matches it, so an operator cannot read a preview of
	// one migration and commit a different one by editing a field afterwards.
	previewedFor := ui.UseState("")
	result := ui.UseState[*adminMigrationResult](nil)
	status := ui.UseState("")
	busy := ui.UseState(false)

	ui.UseEffect(func() func() { ensureManageCSS(); return nil }, "cf-admin-css")

	form := func() migrateForm {
		return migrateForm{
			Mode:              mode.Get(),
			SourceUserID:      strings.TrimSpace(sourceUser.Get()),
			TargetUserID:      p.targetUserID,
			WorkspaceID:       strings.TrimSpace(sourceWS.Get()),
			TargetWorkspaceID: strings.TrimSpace(targetWS.Get()),
			Policy:            policy.Get(),
		}
	}
	fingerprint := func() string {
		f := form()
		return strings.Join([]string{f.Mode, f.SourceUserID, f.TargetUserID, f.WorkspaceID, f.TargetWorkspaceID, f.Policy}, "|")
	}

	toggle := ui.UseEvent(func() { open.Set(!open.Get()) })
	onMode := ui.UseEvent(func(v string) { mode.Set(v); preview.Set(nil); previewedFor.Set("") })
	onPolicy := ui.UseEvent(func(v string) { policy.Set(v); preview.Set(nil); previewedFor.Set("") })
	onSourceUser := ui.UseEvent(func(v string) { sourceUser.Set(v); previewedFor.Set("") })
	onSourceWS := ui.UseEvent(func(v string) { sourceWS.Set(v); previewedFor.Set("") })
	onTargetWS := ui.UseEvent(func(v string) { targetWS.Set(v); previewedFor.Set("") })
	onConfirm := ui.UseEvent(func(v string) { confirm.Set(v) })

	runPreview := ui.UseEvent(func() {
		if busy.Get() {
			return
		}
		busy.Set(true)
		status.Set("")
		result.Set(nil)
		snapshot := fingerprint()
		go func() {
			defer busy.Set(false)
			p2, err := postMigratePreview(p.token, form())
			if err != nil {
				status.Set("Preview failed: " + err.Error())
				return
			}
			preview.Set(p2)
			previewedFor.Set(snapshot)
		}()
	})

	runCommit := ui.UseEvent(func() {
		if busy.Get() {
			return
		}
		busy.Set(true)
		status.Set("")
		go func() {
			defer busy.Set(false)
			f := form()
			f.Confirm = strings.TrimSpace(confirm.Get())
			r, err := postMigrateCommit(p.token, f)
			if err != nil {
				status.Set("Migration failed: " + err.Error())
				return
			}
			result.Set(r)
			preview.Set(nil)
			previewedFor.Set("")
			confirm.Set("")
			status.Set("Migration committed.")
		}()
	})

	rollback := ui.UseEvent(func() {
		r := result.Get()
		if r == nil || busy.Get() {
			return
		}
		busy.Set(true)
		go func() {
			defer busy.Set(false)
			if err := postMigrateRollback(p.token, r.TargetUserID, r.WorkspaceID, r.ArchivedVersion); err != nil {
				status.Set("Rollback failed: " + err.Error())
				return
			}
			status.Set("Rolled back to the archived copy.")
			result.Set(nil)
		}()
	})

	if !open.Get() {
		return Div(css.Class("action-card"), Attr("data-testid", "admin-migrate-card"),
			Div(css.Class("action-desc"),
				Text("Move a workspace from another account to this one, replace this account's copy, or merge the two.")),
			Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-migrate-open"), OnClick(toggle), Text("Migrate a workspace…")),
		)
	}

	// The workspace the operator must retype: the one that CHANGES. For a
	// transfer that is the source (it changes owner); for replace and merge it
	// is the target (its contents change).
	changing := strings.TrimSpace(sourceWS.Get())
	if mode.Get() != "transfer" {
		changing = strings.TrimSpace(targetWS.Get())
	}
	stale := previewedFor.Get() != fingerprint()
	canCommit := preview.Get() != nil && !preview.Get().Blocked && !stale &&
		changing != "" && strings.TrimSpace(confirm.Get()) == changing && !busy.Get()

	return Div(css.Class("action-card"), Attr("data-testid", "admin-migrate-card"),
		Div(css.Class("users-toolbar"),
			H2(css.Class("table-title"), Text("Migrate a workspace")),
			Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-migrate-close"), OnClick(toggle), Text("Close")),
		),
		Div(css.Class("action-desc"),
			Text("Both accounts must be suspended first — a device still syncing can undo the move. Everything replaced is archived and can be rolled back.")),

		Div(css.Class("field-row"),
			Label(Attr("for", "mig-mode"), Text("What to do")),
			Select(Attr("id", "mig-mode"), Attr("data-testid", "admin-migrate-mode"), OnChange(onMode),
				Option(Value("transfer"), SelectedIf(mode.Get() == "transfer"), Text("Transfer — give this account the source's workspace")),
				Option(Value("replace"), SelectedIf(mode.Get() == "replace"), Text("Replace — overwrite this account's copy with the source's")),
				Option(Value("merge"), SelectedIf(mode.Get() == "merge"), Text("Merge — keep the records from both")),
			),
		),
		Div(css.Class("field-row"),
			Label(Attr("for", "mig-source-user"), Text("Source account id")),
			Input(Attr("id", "mig-source-user"), Attr("data-testid", "admin-migrate-source-user"),
				Value(sourceUser.Get()), OnInput(onSourceUser)),
		),
		Div(css.Class("field-row"),
			Label(Attr("for", "mig-source-ws"), Text("Source workspace id")),
			Input(Attr("id", "mig-source-ws"), Attr("data-testid", "admin-migrate-source-ws"),
				Value(sourceWS.Get()), OnInput(onSourceWS)),
		),
		If(mode.Get() != "transfer", Fragment(
			Div(css.Class("field-row"),
				Label(Attr("for", "mig-target-ws"), Text("This account's workspace id (the one that changes)")),
				Input(Attr("id", "mig-target-ws"), Attr("data-testid", "admin-migrate-target-ws"),
					Value(targetWS.Get()), OnInput(onTargetWS)),
			),
		)),
		If(mode.Get() == "merge", Div(css.Class("field-row"),
			Label(Attr("for", "mig-policy"), Text("When both sides have the same record, changed differently")),
			Select(Attr("id", "mig-policy"), Attr("data-testid", "admin-migrate-policy"), OnChange(onPolicy),
				Option(Value("prefer-target"), SelectedIf(policy.Get() == "prefer-target"), Text("Keep this account's version")),
				Option(Value("prefer-source"), SelectedIf(policy.Get() == "prefer-source"), Text("Take the source's version")),
			),
		)),

		Div(css.Class("approval-actions"),
			Button(Type("button"), css.Class("btn btn-secondary"),
				Attr("data-testid", "admin-migrate-preview"), Disabled(busy.Get()),
				OnClick(runPreview), Text("Preview")),
		),
		If(status.Get() != "", Div(css.Class("status-banner"), Attr("role", "status"),
			Attr("data-testid", "admin-migrate-status"), Text(status.Get()))),

		migratePreviewPanel(preview.Get(), stale),
		If(preview.Get() != nil && !preview.Get().Blocked, Fragment(
			Div(css.Class("field-row"),
				Label(Attr("for", "mig-confirm"), Text("Type the id of the workspace that will change: "+changing)),
				Input(Attr("id", "mig-confirm"), Attr("data-testid", "admin-migrate-confirm"),
					Value(confirm.Get()), OnInput(onConfirm)),
			),
			Div(css.Class("approval-actions"),
				Button(Type("button"), css.Class("btn btn-primary"),
					Attr("data-testid", "admin-migrate-commit"), Disabled(!canCommit),
					OnClick(runCommit), Text("Commit migration")),
			),
		)),
		migrateResultPanel(result.Get(), rollback, busy.Get()),
	)
}

// migratePreviewPanel renders what the migration would do, or why it cannot.
func migratePreviewPanel(p *adminMigrationPreview, stale bool) ui.Node {
	if p == nil {
		return Fragment()
	}
	if p.Blocked {
		return Div(css.Class("detail-card"), Attr("data-testid", "admin-migrate-blocked"),
			Div(css.Class("action-desc"), Text("This migration cannot run: "+p.Reason)))
	}
	rows := []ui.Node{
		detailRow("Source workspace", p.WorkspaceName+" ("+p.WorkspaceID+")"),
		detailRow("Source data", fmt.Sprintf("%s · v%d · %s", formatBytes(int64(p.SourceBytes)), p.SourceVersion, trimDate(p.SourceUpdated))),
		detailRow("Source attachments", fmt.Sprintf("%d", p.SourceBlobs)),
	}
	if p.TargetExists {
		rows = append(rows,
			detailRow("Target workspace", p.TargetWorkspaceID),
			detailRow("Target data", fmt.Sprintf("%s · v%d · %s", formatBytes(int64(p.TargetBytes)), p.TargetVersion, trimDate(p.TargetUpdated))),
			detailRow("Target attachments", fmt.Sprintf("%d", p.TargetBlobs)),
		)
	}
	var mergeRows []ui.Node
	if p.Merge != nil {
		mergeRows = append(mergeRows,
			detailRow("Records added", fmt.Sprintf("%d", p.Merge.TotalAdded)),
			detailRow("Records in conflict", fmt.Sprintf("%d", p.Merge.Conflicts)))
		for _, c := range p.Merge.Collections {
			if c.Added == 0 && c.Conflicts == 0 {
				continue
			}
			mergeRows = append(mergeRows, detailRow(c.Name,
				fmt.Sprintf("+%d added · %d in conflict · %d total", c.Added, c.Conflicts, c.Total)))
		}
	}
	var warnRows []ui.Node
	for _, w := range p.Warnings {
		warnRows = append(warnRows, Div(css.Class("action-desc"), Attr("data-testid", "admin-migrate-warning"), Text("⚠ "+w)))
	}
	return Div(Attr("data-testid", "admin-migrate-preview-panel"),
		Div(css.Class("section-title"), Text("What this will do")),
		// A preview the form has drifted away from is worse than none: it
		// describes a migration nobody is about to run.
		If(stale, Div(css.Class("status-banner"), Attr("data-testid", "admin-migrate-stale"),
			Text("You changed the form after this preview. Preview again before committing."))),
		Div(css.Class("detail-card"), rows),
		If(len(mergeRows) > 0, Fragment(
			Div(css.Class("section-title"), Text("Merge result")),
			Div(css.Class("detail-card"), Attr("data-testid", "admin-migrate-merge-counts"), mergeRows),
		)),
		If(len(warnRows) > 0, Div(css.Class("action-card action-danger"), warnRows)),
	)
}

// migrateResultPanel reports a committed migration and offers the undo.
func migrateResultPanel(r *adminMigrationResult, onRollback ui.Handler, busy bool) ui.Node {
	if r == nil {
		return Fragment()
	}
	rows := []ui.Node{
		detailRow("Mode", r.Mode),
		detailRow("Workspace", r.WorkspaceID),
		detailRow("From → to", r.SourceUserID+" → "+r.TargetUserID),
		detailRow("Committed", trimDate(r.CommittedAt)),
	}
	if r.Merge != nil {
		rows = append(rows, detailRow("Records added", fmt.Sprintf("%d", r.Merge.TotalAdded)))
		rows = append(rows, detailRow("Records in conflict", fmt.Sprintf("%d", r.Merge.Conflicts)))
	}
	return Div(Attr("data-testid", "admin-migrate-result"),
		Div(css.Class("section-title"), Text("Committed")),
		Div(css.Class("detail-card"), rows),
		// Rollback is offered right here rather than filed away somewhere,
		// because the moment an operator discovers a migration was wrong is the
		// moment they are looking at its result.
		If(r.ArchivedVersion > 0, Div(css.Class("action-card"),
			Div(css.Class("action-desc"), Text("The copy this replaced was archived. You can put it back.")),
			Button(Type("button"), css.Class("btn btn-danger"),
				Attr("data-testid", "admin-migrate-rollback"), Disabled(busy),
				OnClick(onRollback), Text("Roll back to the archived copy")),
		)),
	)
}
