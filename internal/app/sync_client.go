//go:build js && wasm

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
)

const syncMetaPrefix = "cashflux:sync-meta:"

type syncMeta struct {
	UpdatedAt string `json:"updatedAt,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Version   int64  `json:"version,omitempty"`
}

var syncPushMu sync.Mutex

func startBackendSync() {
	pullActiveWorkspaceFromBackend(true)
	cb := js.FuncOf(func(js.Value, []js.Value) any {
		if js.Global().Get("document").Get("visibilityState").String() == "visible" {
			pullActiveWorkspaceFromBackend(true)
		}
		return nil
	})
	js.Global().Call("addEventListener", "visibilitychange", cb)
	js.Global().Call("addEventListener", "focus", cb)
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
	go func() {
		syncPushMu.Lock()
		defer syncPushMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: pr.ServerURL, Token: pr.ServerToken})
		if err != nil {
			logSyncError("backend sync dial failed", err)
			return
		}
		defer conn.Close()
		var resp backendrpc.PutWorkspaceResponse
		err = conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, backendrpc.PutWorkspaceRequest{
			Workspace: backendrpc.Workspace{
				ID:    w.ID,
				Name:  w.Name,
				Color: w.Color,
				Sort:  workspaceSort(r, w.ID),
			},
			Dataset:         dataset,
			ClientUpdatedAt: updatedAt.UTC().Format(time.RFC3339Nano),
		}, &resp, backendrpc.JSONCallOptions()...)
		if err != nil {
			logSyncError("backend sync push failed", err)
			return
		}
		if !resp.Accepted {
			if app := appstate.Default; app != nil {
				app.Log().Warn("backend sync push rejected; newer server snapshot available", "workspace", w.ID)
			}
			return
		}
		saveSyncMeta(w.ID, syncMeta{UpdatedAt: resp.UpdatedAt, Version: resp.Version, Hash: hash})
	}()
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
