// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/datasetmerge"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// The two operations C695 asks for, from the user's own device.
//
// The ticket's issue statement is exact about the failure: "the user intended
// the current browser dataset to replace the old account's dataset, but the only
// available path creates another device account". So after a rebind picks the
// workspace this data belongs to, the user still has to say what should happen
// when BOTH copies have records — and the two honest answers are "mine is the
// right one" and "keep everything from both".
//
// Neither is offered blind. Each is preceded by a comparison of what is on each
// side, each exports a local backup first, and each states what it will do in
// records rather than in bytes — because "1.2 MB replacing 900 KB" tells nobody
// whether they are about to lose a month of transactions.

// remoteComparison is what the two copies look like side by side, for the
// decision the user is about to make.
type remoteComparison struct {
	WorkspaceID string
	// RemoteFound is false when the target workspace has no snapshot yet, in
	// which case there is nothing to replace or merge and an ordinary push does.
	RemoteFound bool
	// Merge is what a merge WOULD do, computed by performing one and throwing it
	// away. Estimating here would describe a different operation than the one
	// about to run.
	Merge datasetmerge.Report
	// LocalBytes and RemoteBytes are shown as a secondary detail only.
	LocalBytes  int
	RemoteBytes int
}

// compareWithRemote fetches the target workspace's snapshot and reports what
// merging into it would do.
func compareWithRemote(workspaceID string, policy datasetmerge.Policy) (remoteComparison, error) {
	out := remoteComparison{WorkspaceID: workspaceID}
	app := appstate.Default
	if app == nil {
		return out, fmt.Errorf("no local dataset is loaded")
	}
	local, err := app.ExportJSONRedacted()
	if err != nil {
		return out, err
	}
	out.LocalBytes = len(local)

	pr := uistate.LoadPrefs().Normalize()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var resp backendrpc.GetWorkspaceResponse
	if err := invokeAuthed(ctx, pr, backendrpc.MethodSyncGetWorkspace,
		backendrpc.GetWorkspaceRequest{ID: workspaceID}, &resp); err != nil {
		return out, err
	}
	if !resp.Found || len(resp.Dataset) == 0 {
		return out, nil
	}
	out.RemoteFound = true
	out.RemoteBytes = len(resp.Dataset)
	// The remote is the TARGET of a merge here: the user is deciding what to add
	// to the copy the account already has.
	_, report, err := datasetmerge.Merge(resp.Dataset, local, policy)
	if err != nil {
		return out, err
	}
	out.Merge = report
	return out, nil
}

// replaceRemoteWithLocal force-pushes this device's dataset over the target
// workspace's copy.
//
// Force bypasses the last-write-wins staleness check, which is correct only
// because the user has explicitly said this copy is the right one — the same
// reasoning, and the same mechanism, as resolving an LWW conflict with "keep my
// changes". The caller exports a backup first.
func replaceRemoteWithLocal(workspaceID string) error {
	app := appstate.Default
	if app == nil {
		return fmt.Errorf("no local dataset is loaded")
	}
	local, err := app.ExportJSONRedacted()
	if err != nil {
		return err
	}
	return pushDatasetToWorkspace(workspaceID, local)
}

// mergeWithRemote pulls the target's snapshot, merges this device's records into
// it, applies the result locally, and pushes it.
//
// The merged copy is applied LOCALLY FIRST and pushed only if that succeeded. A
// push that lands while the local import fails would leave the device holding a
// dataset the server has already moved past — the device would then be the one
// out of date about its own merge.
func mergeWithRemote(workspaceID string, policy datasetmerge.Policy) (datasetmerge.Report, error) {
	var report datasetmerge.Report
	app := appstate.Default
	if app == nil {
		return report, fmt.Errorf("no local dataset is loaded")
	}
	local, err := app.ExportJSONRedacted()
	if err != nil {
		return report, err
	}
	pr := uistate.LoadPrefs().Normalize()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var resp backendrpc.GetWorkspaceResponse
	if err := invokeAuthed(ctx, pr, backendrpc.MethodSyncGetWorkspace,
		backendrpc.GetWorkspaceRequest{ID: workspaceID}, &resp); err != nil {
		return report, err
	}
	if !resp.Found || len(resp.Dataset) == 0 {
		// Nothing on the other side: a merge into nothing is just this copy.
		return report, pushDatasetToWorkspace(workspaceID, local)
	}
	merged, report, err := datasetmerge.Merge(resp.Dataset, local, policy)
	if err != nil {
		return report, err
	}
	if err := app.ImportJSON(merged); err != nil {
		return report, fmt.Errorf("merged copy could not be applied locally: %w", err)
	}
	// Re-render the surfaces reading the dataset; the autosave path writes it
	// through to storage on the same revision bump.
	uistate.BumpDataRevision()
	return report, pushDatasetToWorkspace(workspaceID, merged)
}

// pushDatasetToWorkspace sends a dataset as the workspace's authoritative copy.
func pushDatasetToWorkspace(workspaceID string, dataset []byte) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("no workspace to write to")
	}
	pr := uistate.LoadPrefs().Normalize()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := workspaceID
	if w, ok := loadRegistry().Get(workspaceID); ok && strings.TrimSpace(w.Name) != "" {
		name = w.Name
	}
	prepared, err := prepareBackendSyncDataset(ctx, pr.ServerURL, effectiveServerToken(pr), workspaceID, dataset)
	if err != nil {
		return err
	}
	var resp backendrpc.PutWorkspaceResponse
	if err := invokeAuthed(ctx, pr, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
		Workspace: backendrpc.Workspace{
			ID:       workspaceID,
			Name:     name,
			DeviceID: syncDeviceID(),
		},
		Dataset:         prepared,
		ClientUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		// The user has explicitly chosen this copy, which is the same
		// justification the "keep my changes" conflict resolution uses.
		Force: true,
	}, &resp); err != nil {
		return err
	}
	saveSyncMeta(workspaceID, syncMeta{
		UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: datasetHash(dataset),
	})
	setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	return nil
}
