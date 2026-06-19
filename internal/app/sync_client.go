//go:build js && wasm

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
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
}

func pushActiveWorkspaceToBackend(dataset []byte, updatedAt time.Time) {
	pr := uistate.LoadPrefs().Normalize()
	if strings.TrimSpace(pr.ServerURL) == "" || strings.TrimSpace(pr.ServerToken) == "" {
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
	if strings.TrimSpace(pr.ServerURL) == "" || strings.TrimSpace(pr.ServerToken) == "" {
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
			var resp backendrpc.PutWorkspaceResponse
			err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
				Workspace: backendrpc.Workspace{
					ID:       item.WorkspaceID,
					Name:     item.Name,
					Color:    item.Color,
					Sort:     item.Sort,
					DeviceID: item.DeviceID,
				},
				Dataset:         []byte(item.Dataset),
				ClientUpdatedAt: item.ClientUpdatedAt,
			}, &resp, backendrpc.JSONCallOptions()...)
			if err != nil {
				item.LastAttemptError = err.Error()
				upsertQueuedSyncMutation(item)
				setSyncStatus(syncStatus{State: "error", Pending: len(loadSyncQueue()), Message: "sync failed"})
				logSyncError("backend sync push failed", err)
				return
			}
			removeQueuedSyncMutation(item.WorkspaceID, item.Hash)
			if !resp.Accepted {
				setSyncStatus(syncStatus{State: "conflict", Pending: len(loadSyncQueue()), Message: "newer server snapshot available"})
				if app := appstate.Default; app != nil {
					app.Log().Warn("backend sync push rejected; newer server snapshot available", "workspace", item.WorkspaceID)
				}
				continue
			}
			saveSyncMeta(item.WorkspaceID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: item.Hash})
		}
		setSyncStatus(syncStatus{State: "synced", Pending: len(loadSyncQueue()), LastSyncedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	}()
}

func startBackendWatch() {
	pr := uistate.LoadPrefs().Normalize()
	if strings.TrimSpace(pr.ServerURL) == "" || strings.TrimSpace(pr.ServerToken) == "" {
		return
	}
	go func() {
		for {
			ctx := context.Background()
			conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
			if err != nil {
				logSyncError("backend sync watch dial failed", err)
				time.Sleep(10 * time.Second)
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
				readBackendWatch(stream)
			} else {
				logSyncError("backend sync watch failed", err)
			}
			_ = conn.Close()
			time.Sleep(3 * time.Second)
		}
	}()
}

func readBackendWatch(stream grpc.ClientStream) {
	for {
		var event backendrpc.WatchWorkspacesResponse
		if err := stream.RecvMsg(&event); err != nil {
			logSyncError("backend sync watch closed", err)
			return
		}
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
	if strings.TrimSpace(pr.ServerURL) == "" || strings.TrimSpace(pr.ServerToken) == "" {
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
		if err := app.ImportJSON(resp.Dataset); err != nil {
			logSyncError("backend sync import failed", err)
			return
		}
		lsSet(datasetStoreKey, string(resp.Dataset))
		hadLocalDataset = true
		saveSyncMeta(w.ID, syncMeta{UpdatedAt: resp.Workspace.UpdatedAt, Version: resp.Workspace.Version, Hash: datasetHash(resp.Dataset)})
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
		status.State = "synced"
	}
	if data, err := json.Marshal(status); err == nil {
		lsSet(syncStatusKey, string(data))
	}
}

func loadSyncStatus() syncStatus {
	var status syncStatus
	if raw := lsGet(syncStatusKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &status)
	}
	if status.State == "" {
		if pending := len(loadSyncQueue()); pending > 0 {
			status.State = "offline"
			status.Pending = pending
		} else {
			status.State = "synced"
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
