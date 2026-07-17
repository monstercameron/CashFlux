// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/backoff"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"github.com/monstercameron/CashFlux/internal/syncstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
	"google.golang.org/grpc"
)

const syncMetaPrefix = "cashflux:sync-meta:"
const syncDeviceIDKey = "cashflux:sync-device-id"
const syncQueueKey = "cashflux:sync-queue"
const syncStatusKey = "cashflux:sync-status"

// syncConflictPrefix stores the LAST local dataset that lost an LWW conflict (the
// server held a newer snapshot), keyed by workspace. C309: this is the recoverable
// backup so a rejected local edit is never silently lost — the user can restore it
// from Settings → Cloud sync. One slot per workspace (the latest loser).
const syncConflictPrefix = "cashflux:sync-conflict:"

type syncMeta struct {
	UpdatedAt string `json:"updatedAt,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Version   int64  `json:"version,omitempty"`
}

type queuedSyncMutation struct {
	WorkspaceID      string `json:"workspaceId"`
	Name             string `json:"name,omitempty"`
	Color            string `json:"color,omitempty"`
	Sort             int    `json:"sort,omitempty"`
	DeviceID         string `json:"deviceId,omitempty"`
	Dataset          string `json:"dataset"`
	ClientUpdatedAt  string `json:"clientUpdatedAt"`
	Hash             string `json:"hash"`
	LastAttemptError string `json:"lastAttemptError,omitempty"`
}

type syncStatus struct {
	State        string `json:"state"`
	Pending      int    `json:"pending,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
	Message      string `json:"message,omitempty"`
}

var syncPushMu sync.Mutex

func startBackendSync() {
	// Don't wire up auto-sync (or its visibility/focus/online listeners) when the
	// backend is off or unconfigured — otherwise the app dials a websocket on load
	// and surfaces connection errors the user can't act on (C81 follow-up).
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		return
	}
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
	startBackendWatch()
	cb := js.FuncOf(func(js.Value, []js.Value) any {
		if js.Global().Get("document").Get("visibilityState").String() == "visible" {
			flushBackendSyncQueue()
			pullActiveWorkspaceFromBackend(true)
		}
		return nil
	})
	js.Global().Call("addEventListener", "visibilitychange", cb)
	js.Global().Call("addEventListener", "focus", cb)
	js.Global().Call("addEventListener", "online", js.FuncOf(func(js.Value, []js.Value) any {
		flushBackendSyncQueue()
		return nil
	}))
	// C323: reflect going offline immediately. Without this only "online" was wired,
	// so a dropped connection left the chip on its last (often "synced") state until
	// the next failed dial. Mark any pending work as queued/offline right away.
	js.Global().Call("addEventListener", "offline", js.FuncOf(func(js.Value, []js.Value) any {
		st := loadSyncStatus()
		st.State = "offline"
		st.Pending = len(loadSyncQueue())
		setSyncStatus(st)
		return nil
	}))
}

func pushActiveWorkspaceToBackend(dataset []byte, updatedAt time.Time) {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	hash := datasetHash(dataset)
	meta := loadSyncMeta(w.ID)
	if meta.Hash == hash {
		return
	}
	enqueueSyncMutation(queuedSyncMutation{
		WorkspaceID:     w.ID,
		Name:            w.Name,
		Color:           w.Color,
		Sort:            workspaceSort(r, w.ID),
		DeviceID:        syncDeviceID(),
		Dataset:         string(dataset),
		ClientUpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano),
		Hash:            hash,
	})
	flushBackendSyncQueue()
}

func requestBackendSyncNow() {
	flushBackendSyncQueue()
	pullActiveWorkspaceFromBackend(true)
}

func flushBackendSyncQueue() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	go func() {
		syncPushMu.Lock()
		defer syncPushMu.Unlock()
		queue := loadSyncQueue()
		if len(queue) == 0 {
			setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			return
		}
		setSyncStatus(syncStatus{State: "syncing", Pending: len(queue)})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
		if err != nil {
			setSyncStatus(syncStatus{State: "offline", Pending: len(queue), Message: "backend unavailable"})
			logSyncError("backend sync dial failed", err)
			return
		}
		defer conn.Close()
		for _, item := range queue {
			dataset, err := prepareBackendSyncDataset(ctx, pr.ServerURL, pr.ServerToken, item.WorkspaceID, []byte(item.Dataset))
			if err != nil {
				item.LastAttemptError = err.Error()
				upsertQueuedSyncMutation(item)
				setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: "artifact blob upload failed"})
				logSyncError("backend artifact blob upload failed", err)
				return
			}
			var resp backendrpc.PutWorkspaceResponse
			err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
				Workspace: backendrpc.Workspace{
					ID:       item.WorkspaceID,
					Name:     item.Name,
					Color:    item.Color,
					Sort:     item.Sort,
					DeviceID: item.DeviceID,
				},
				Dataset:         dataset,
				ClientUpdatedAt: item.ClientUpdatedAt,
			}, &resp, backendrpc.JSONCallOptions()...)
			if err != nil {
				item.LastAttemptError = err.Error()
				upsertQueuedSyncMutation(item)
				setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: "sync failed"})
				logSyncError("backend sync push failed", err)
				return
			}
			if !resp.Accepted {
				// LWW resolution: the server holds a newer snapshot, so this push lost.
				// C309: do NOT silently drop the local edit. Before removing it from the
				// active queue (which must happen, or it would re-push and re-lose every
				// cycle — an infinite conflict loop), stash the rejected local dataset to
				// a recoverable per-workspace backup so the user can restore it. Then tell
				// them plainly (§7.11) and pull the newer server copy so the UI is current.
				saveConflictBackup(item)
				removeQueuedSyncMutation(item.WorkspaceID, item.Hash)
				setSyncStatus(syncStatus{State: "conflict", Pending: len(loadSyncQueue()), Message: "newer server snapshot available"})
				uistate.PostNotice(uistate.T("sync.conflictBackedUp"), false)
				if app := appstate.Default; app != nil {
					app.Log().Warn("backend sync push rejected; local edit backed up, newer server snapshot pulled", "workspace", item.WorkspaceID)
				}
				continue
			}
			// Accepted: only now is it safe to drop the local mutation from the queue.
			removeQueuedSyncMutation(item.WorkspaceID, item.Hash)
			saveSyncMeta(item.WorkspaceID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: item.Hash})
		}
		setSyncStatus(syncStatus{State: "synced", Pending: len(loadSyncQueue()), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	}()
}

func startBackendWatch() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	go func() {
		// C322: exponential backoff + jitter (2s→120s cap) instead of fixed
		// 10s/3s sleeps, so a flapping network doesn't hammer the backend and many
		// clients don't reconnect in lockstep.
		const baseDelay, capDelay, jitterFrac = 2 * time.Second, 120 * time.Second, 0.3
		// healthyAfter is how long a stream must stay up (absent any received
		// message) to count as a healthy connection worth resetting the backoff for
		// — the thrash guard against a stream that opens then instantly errors.
		const healthyAfter = 30 * time.Second
		attempt := 0
		sleepBackoff := func() {
			d := backoff.Jitter(backoff.Delay(attempt, baseDelay, capDelay), jitterFrac, rand.Float64())
			time.Sleep(d)
			attempt++
		}
		// firstConnect skips the reconcile pull on the very first successful
		// subscribe, because startBackendSync already pulled at boot — only
		// RE-connects need to reconcile the gap.
		firstConnect := true
		for {
			ctx := context.Background()
			conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
			if err != nil {
				logSyncError("backend sync watch dial failed", err)
				sleepBackoff()
				continue
			}
			stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodSyncWatchWorkspaces, backendrpc.JSONCallOptions()...)
			if err == nil {
				err = stream.SendMsg(&backendrpc.WatchWorkspacesRequest{IncludeDeleted: true})
			}
			if err == nil {
				err = stream.CloseSend()
			}
			if err == nil {
				// Reconcile on every RE-subscribe: the server streams only FUTURE
				// events and silently drops on a full send buffer, so a client that
				// was briefly disconnected (or whose buffer overflowed) would miss
				// other devices' changes with no signal. Pulling the active workspace
				// now closes that gap — the push stream alone is best-effort.
				if !firstConnect {
					pullActiveWorkspaceFromBackend(true)
				}
				firstConnect = false
				connectedAt := time.Now()
				received := readBackendWatch(stream)
				// Reset the backoff only when the stream proved healthy (delivered a
				// message or stayed up long enough); an immediate error keeps the
				// backoff climbing instead of reconnecting at the floor forever.
				if syncstate.ShouldResetBackoff(received, time.Since(connectedAt), healthyAfter) {
					attempt = 0
				}
			} else {
				logSyncError("backend sync watch failed", err)
			}
			_ = conn.Close()
			sleepBackoff()
		}
	}()
}

// readBackendWatch reads live workspace events until the stream ends, pulling the
// active workspace whenever another device changes it. It returns whether it
// received at least one event, which the reconnect loop uses as a health signal
// (a stream that delivered data was healthy even if it was short-lived).
func readBackendWatch(stream grpc.ClientStream) (received bool) {
	for {
		var event backendrpc.WatchWorkspacesResponse
		if err := stream.RecvMsg(&event); err != nil {
			logSyncError("backend sync watch closed", err)
			return received
		}
		received = true
		if strings.TrimSpace(event.Workspace.ID) == "" || event.Workspace.DeviceID == syncDeviceID() {
			continue
		}
		r := loadRegistry()
		active, ok := r.Active()
		if ok && active.ID == event.Workspace.ID {
			pullActiveWorkspaceFromBackend(true)
		}
	}
}

func pullActiveWorkspaceFromBackend(reloadOnApply bool) {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
		if err != nil {
			logSyncError("backend sync dial failed", err)
			return
		}
		defer conn.Close()
		var resp backendrpc.GetWorkspaceResponse
		err = conn.Invoke(ctx, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: w.ID}, &resp, backendrpc.JSONCallOptions()...)
		if err != nil {
			logSyncError("backend sync pull failed", err)
			setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: "pull failed"})
			return
		}
		if !resp.Found || len(resp.Dataset) == 0 {
			return
		}
		dataset, err := hydrateBackendSyncDataset(ctx, pr.ServerURL, pr.ServerToken, w.ID, resp.Dataset)
		if err != nil {
			logSyncError("backend artifact blob download failed", err)
			setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: "artifact blob download failed"})
			return
		}
		meta := loadSyncMeta(w.ID)
		localUpdatedAt, hasLocalMeta := parseSyncMetaTime(meta)
		remoteUpdatedAt, err := time.Parse(time.RFC3339Nano, resp.Workspace.UpdatedAt)
		if err != nil {
			logSyncError("backend sync timestamp parse failed", err)
			return
		}
		if !syncstate.ShouldApplyRemote(localUpdatedAt, hasLocalMeta, hadLocalDataset, remoteUpdatedAt, true) {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.ImportJSON(dataset); err != nil {
			logSyncError("backend sync import failed", err)
			return
		}
		lsSet(datasetStoreKey, string(dataset))
		// Deliberate same-tab dataset replacement: advance the cross-tab generation
		// (other tabs must stop overwriting) and this tab's own write entitlement.
		datasetMyGen = bumpDatasetGen()
		hadLocalDataset = true
		saveSyncMeta(w.ID, syncMeta{UpdatedAt: resp.Workspace.UpdatedAt, Version: resp.Workspace.Version, Hash: datasetHash(dataset)})
		setSyncStatus(syncStatus{State: "synced", Pending: len(loadSyncQueue()), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		if reloadOnApply {
			reloadPage()
		}
	}()
}

func workspaceSort(r workspace.Registry, id string) int {
	for i, w := range r.Workspaces {
		if w.ID == id {
			return i
		}
	}
	return 0
}

func datasetHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func syncMetaKey(workspaceID string) string { return syncMetaPrefix + workspaceID }

func loadSyncMeta(workspaceID string) syncMeta {
	var meta syncMeta
	if raw := lsGet(syncMetaKey(workspaceID)); raw != "" {
		_ = json.Unmarshal([]byte(raw), &meta)
	}
	return meta
}

func saveSyncMeta(workspaceID string, meta syncMeta) {
	if data, err := json.Marshal(meta); err == nil {
		lsSet(syncMetaKey(workspaceID), string(data))
	}
}

func loadSyncQueue() []queuedSyncMutation {
	var queue []queuedSyncMutation
	if raw := lsGet(syncQueueKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &queue)
	}
	return queue
}

func saveSyncQueue(queue []queuedSyncMutation) {
	if len(queue) == 0 {
		lsRemove(syncQueueKey)
		return
	}
	if data, err := json.Marshal(queue); err == nil {
		lsSet(syncQueueKey, string(data))
	}
}

// saveConflictBackup stores a rejected local mutation as the recoverable backup for
// its workspace (C309). One slot per workspace — the latest conflict overwrites the
// previous, since the user's most recent local edit is the one worth recovering.
func saveConflictBackup(item queuedSyncMutation) {
	if data, err := json.Marshal(item); err == nil {
		lsSet(syncConflictPrefix+item.WorkspaceID, string(data))
	}
}

// loadConflictBackup returns the recoverable local mutation that last lost an LWW
// conflict for a workspace, and whether one exists.
func loadConflictBackup(workspaceID string) (queuedSyncMutation, bool) {
	raw := lsGet(syncConflictPrefix + workspaceID)
	if raw == "" {
		return queuedSyncMutation{}, false
	}
	var item queuedSyncMutation
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return queuedSyncMutation{}, false
	}
	return item, true
}

// clearConflictBackup discards the recoverable backup once the user restores or
// dismisses it.
func clearConflictBackup(workspaceID string) {
	lsRemove(syncConflictPrefix + workspaceID)
}

// restoreConflictBackup re-applies a backed-up local mutation that previously lost an
// LWW conflict: it re-stamps the client timestamp to now (so it wins the next LWW
// round against the snapshot that beat it), re-enqueues it, clears the backup, and
// kicks a flush. Returns false if there is no backup for the workspace. (C309)
func restoreConflictBackup(workspaceID string) bool {
	item, ok := loadConflictBackup(workspaceID)
	if !ok {
		return false
	}
	item.ClientUpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	item.LastAttemptError = ""
	enqueueSyncMutation(item)
	clearConflictBackup(workspaceID)
	flushBackendSyncQueue()
	return true
}

// hasConflictBackup reports whether a recoverable conflict backup exists for the
// active workspace (drives the Settings restore affordance).
func hasConflictBackup(workspaceID string) bool {
	_, ok := loadConflictBackup(workspaceID)
	return ok
}

// resolveConflictKeepLocal re-pushes the stashed local dataset with Force=true
// so the server accepts it unconditionally (bypassing the LWW staleness check),
// then clears the conflict backup and marks sync as settled. Called by
// SyncConflictHost's "Keep my changes" action. (C309 / #464)
func resolveConflictKeepLocal() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	item, ok := loadConflictBackup(w.ID)
	if !ok {
		// Nothing stashed — conflict may have already been resolved; reset status.
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		return
	}
	go func() {
		syncPushMu.Lock()
		defer syncPushMu.Unlock()
		setSyncStatus(syncStatus{State: "syncing"})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Pending: 0, Message: "backend unavailable"})
			logSyncError("conflict resolve-keep dial failed", err)
			return
		}
		defer conn.Close()
		dataset, err := prepareBackendSyncDataset(ctx, pr.ServerURL, pr.ServerToken, item.WorkspaceID, []byte(item.Dataset))
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Message: "artifact upload failed"})
			logSyncError("conflict resolve-keep artifact upload failed", err)
			return
		}
		var resp backendrpc.PutWorkspaceResponse
		err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
			Workspace: backendrpc.Workspace{
				ID:       item.WorkspaceID,
				Name:     item.Name,
				Color:    item.Color,
				Sort:     item.Sort,
				DeviceID: item.DeviceID,
			},
			Dataset:         dataset,
			ClientUpdatedAt: item.ClientUpdatedAt,
			Force:           true, // bypass LWW staleness check — user chose "keep local"
		}, &resp, backendrpc.JSONCallOptions()...)
		if err != nil {
			setSyncStatus(syncStatus{State: "error", Message: "force push failed"})
			logSyncError("conflict resolve-keep force push failed", err)
			return
		}
		// Force=true means the server always accepts; clear the backup and settle.
		clearConflictBackup(item.WorkspaceID)
		saveSyncMeta(item.WorkspaceID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: item.Hash})
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		uistate.PostNotice(uistate.T("sync.conflictResolvedKeepLocal"), false)
	}()
}

// resolveConflictUseServer pulls the current server snapshot, applies it
// locally, and discards the stashed local dataset ONLY after a successful
// import — so the stash is never lost due to a mid-operation failure. Called by
// SyncConflictHost's "Use server version" action. (C309 / #464)
func resolveConflictUseServer() {
	pr := uistate.LoadPrefs().Normalize()
	if !pr.BackendActive() {
		return
	}
	r := loadRegistry()
	w, ok := r.Active()
	if !ok {
		return
	}
	wID := w.ID
	go func() {
		setSyncStatus(syncStatus{State: "syncing"})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
		if err != nil {
			// Revert to conflict so the chip still offers the modal.
			setSyncStatus(syncStatus{State: "conflict", Message: "backend unavailable"})
			logSyncError("conflict resolve-server dial failed", err)
			return
		}
		defer conn.Close()
		var resp backendrpc.GetWorkspaceResponse
		err = conn.Invoke(ctx, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: wID}, &resp, backendrpc.JSONCallOptions()...)
		if err != nil {
			setSyncStatus(syncStatus{State: "conflict"})
			logSyncError("conflict resolve-server pull failed", err)
			return
		}
		if !resp.Found || len(resp.Dataset) == 0 {
			// Server has no snapshot — treat as resolved (nothing to pull).
			clearConflictBackup(wID)
			setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			return
		}
		dataset, err := hydrateBackendSyncDataset(ctx, pr.ServerURL, pr.ServerToken, wID, resp.Dataset)
		if err != nil {
			setSyncStatus(syncStatus{State: "conflict"})
			logSyncError("conflict resolve-server hydrate failed", err)
			return
		}
		app := appstate.Default
		if app == nil {
			setSyncStatus(syncStatus{State: "conflict"})
			return
		}
		if err := app.ImportJSON(dataset); err != nil {
			setSyncStatus(syncStatus{State: "conflict"})
			logSyncError("conflict resolve-server import failed", err)
			return
		}
		lsSet(datasetStoreKey, string(dataset))
		// Deliberate same-tab dataset replacement: advance the cross-tab generation
		// (other tabs must stop overwriting) and this tab's own write entitlement.
		datasetMyGen = bumpDatasetGen()
		hadLocalDataset = true
		saveSyncMeta(wID, syncMeta{
			UpdatedAt: resp.Workspace.UpdatedAt,
			Version:   resp.Workspace.Version,
			Hash:      datasetHash(dataset),
		})
		// Only discard the stash after the import has succeeded — the user's local
		// edit is recoverable until this point.
		clearConflictBackup(wID)
		setSyncStatus(syncStatus{State: "synced", LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		uistate.PostNotice(uistate.T("sync.conflictResolvedUseServer"), false)
		reloadPage()
	}()
}

func enqueueSyncMutation(item queuedSyncMutation) {
	upsertQueuedSyncMutation(item)
	setSyncStatus(syncStatus{State: "syncing", Pending: len(loadSyncQueue())})
}

func upsertQueuedSyncMutation(item queuedSyncMutation) {
	queue := loadSyncQueue()
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
	}
	pending = syncstate.UpsertPending(pending, syncstate.PendingMutation{WorkspaceID: item.WorkspaceID, Hash: item.Hash, UpdatedAt: item.ClientUpdatedAt})
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		if p.WorkspaceID == item.WorkspaceID && p.Hash == item.Hash {
			next = append(next, item)
			continue
		}
		for _, q := range queue {
			if q.WorkspaceID == p.WorkspaceID && q.Hash == p.Hash {
				next = append(next, q)
				break
			}
		}
	}
	saveSyncQueue(next)
}

func removeQueuedSyncMutation(workspaceID, hash string) {
	queue := loadSyncQueue()
	pending := make([]syncstate.PendingMutation, 0, len(queue))
	byKey := map[string]queuedSyncMutation{}
	for _, q := range queue {
		pending = append(pending, syncstate.PendingMutation{WorkspaceID: q.WorkspaceID, Hash: q.Hash, UpdatedAt: q.ClientUpdatedAt})
		byKey[q.WorkspaceID+"\x00"+q.Hash] = q
	}
	pending = syncstate.RemovePending(pending, workspaceID, hash)
	next := make([]queuedSyncMutation, 0, len(pending))
	for _, p := range pending {
		if q, ok := byKey[p.WorkspaceID+"\x00"+p.Hash]; ok {
			next = append(next, q)
		}
	}
	saveSyncQueue(next)
}

func setSyncStatus(status syncStatus) {
	if status.State == "" {
		status.State = "local" // C5: unset = local-only, not "synced"
	}
	if data, err := json.Marshal(status); err == nil {
		lsSet(syncStatusKey, string(data))
	}
	// C324: make the chip reactive. setSyncStatus is called from background
	// goroutines (watch/flush/pull); bumping the captured revision atom triggers a
	// re-render so the chip reflects the new state without waiting for an unrelated
	// render. Captured during SyncChip's render; no-op until the chip has mounted.
	if syncStatusCaptured {
		capturedSyncRev.Set(capturedSyncRev.Get() + 1)
	}
}

func loadSyncStatus() syncStatus {
	var status syncStatus
	if raw := lsGet(syncStatusKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &status)
	}
	// C320 (supersedes C5): the chip reflects CLOUD sync. If no backend is configured
	// (the default local-first session, OR a backend that was configured then turned
	// off), there is nothing to report — force an empty state so SyncChip stays
	// invisible. This also discards a stale "synced" left in localStorage from a
	// previously-active backend, which would otherwise read as a false "Synced".
	if !uistate.LoadPrefs().Normalize().BackendActive() {
		status.State = ""
		return status
	}
	if status.State == "" {
		if pending := len(loadSyncQueue()); pending > 0 {
			status.State = "offline"
			status.Pending = pending
		} else {
			// C5: a session that never cloud-synced is LOCAL, not "synced" — defaulting
			// to "synced" rendered a misleading "Synced" chip on a local-first session
			// (and defeated SyncChip's "invisible until cloud sync is in use" intent).
			// Real cloud syncs set State="synced" explicitly (see setSyncStatus callers).
			status.State = "local"
		}
	}
	return status
}

func syncStatusLabel() string {
	status := loadSyncStatus()
	switch status.State {
	case "syncing":
		return "Syncing"
	case "offline":
		if status.Pending > 0 {
			return "Offline - " + strconv.Itoa(status.Pending) + " queued"
		}
		return "Offline"
	case "error":
		return "Sync error"
	case "conflict":
		return "Newer server copy available"
	case "local", "":
		// C320: local-first / no backend configured — no cloud "Synced" claim.
		return "Saved on this device"
	default:
		if status.Pending > 0 {
			return strconv.Itoa(status.Pending) + " queued"
		}
		return "Synced"
	}
}

func syncDeviceID() string {
	if id := strings.TrimSpace(lsGet(syncDeviceIDKey)); id != "" {
		return id
	}
	id := ""
	crypto := js.Global().Get("crypto")
	if !crypto.IsUndefined() && !crypto.IsNull() {
		randomUUID := crypto.Get("randomUUID")
		if randomUUID.Type() == js.TypeFunction {
			id = randomUUID.Invoke().String()
		}
	}
	if strings.TrimSpace(id) == "" {
		id = "browser-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	lsSet(syncDeviceIDKey, id)
	return id
}

func parseSyncMetaTime(meta syncMeta) (time.Time, bool) {
	if strings.TrimSpace(meta.UpdatedAt) == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, meta.UpdatedAt)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func logSyncError(msg string, err error) {
	if app := appstate.Default; app != nil && err != nil {
		app.Log().Warn(msg, "err", err)
	}
}
