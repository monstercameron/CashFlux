//go:build js && wasm

package app

import (
	"encoding/json"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/store"
)

// Recovery maintenance cannot precede first paint. Two animation frames give the
// browser a render opportunity before the Go worker starts reading old blobs.
func startRecoveryMaintenance() {
	window := js.Global()
	raf := window.Get("requestAnimationFrame")
	if raf.Type() != js.TypeFunction {
		go func() { time.Sleep(50 * time.Millisecond); runRecoveryMaintenance() }()
		return
	}
	var first, second js.Func
	second = js.FuncOf(func(js.Value, []js.Value) any { second.Release(); go runRecoveryMaintenance(); return nil })
	first = js.FuncOf(func(js.Value, []js.Value) any {
		first.Release()
		window.Call("requestAnimationFrame", second)
		return nil
	})
	window.Call("requestAnimationFrame", first)
}

func liveRecoveryVersions() browserstore.RecoveryLive {
	snapshot, err := readDurableDatasetSnapshot()
	if err != nil {
		return browserstore.RecoveryLive{}
	}
	portable, _, err := recoverDatasetSnapshot(snapshot)
	if err != nil {
		return browserstore.RecoveryLive{}
	}
	group := recoveryScopeGroup("stale", checkpointHash(snapshot.workspace), checkpointHash(snapshot.Values[datasetOwnerKey]))
	return browserstore.RecoveryLive{Versions: map[string][]string{group: recoveryRecordVersions(portable)}, Snapshot: snapshot.SecureSnapshot, Conditions: []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey}}
}

func runRecoveryMaintenance() {
	slog.Info("recovery maintenance", "status", "started")
	browserstore.PruneRecovery(describeRecoveryPayload, liveRecoveryVersions(), func(stats browserstore.RecoveryStats, err error) {
		if err != nil {
			slog.Warn("recovery maintenance paused; remaining copies were kept", "error", err, "removed", stats.Removed, "bytes_freed", stats.BytesFreed)
			return
		}
		empty, err := browserstore.PruneEmptyJournalRecovery()
		if err != nil {
			slog.Warn("empty recovery cleanup paused", "error", err)
		}
		stats.Removed += empty.Removed
		stats.BytesFreed += empty.BytesFreed
		clearEmptyQuarantineState()
		slog.Info("recovery maintenance", "status", "complete", "removed", stats.Removed, "bytes_freed", stats.BytesFreed)
	})
}

func lazyRecoveryDownload() []byte {
	copies := map[string]string{}
	workspace := recordWorkspace()
	owner := browserstore.GetString(datasetOwnerKey)
	for _, key := range browserstore.Keys() {
		if (!strings.HasPrefix(key, quarantinePrefix) && !strings.HasPrefix(key, journalRecoveryPrefix)) || strings.HasSuffix(key, ":reason") {
			continue
		}
		raw, present, err := browserstore.GetSecure(key)
		if err != nil || !present || raw == "" {
			continue
		}
		var copy recordRecovery
		if json.Unmarshal([]byte(raw), &copy) == nil && copy.Format == "v1" {
			if !recoveryCopyMatchesScope(copy, workspace, owner, hostedDocument()) {
				continue
			}
		} else if hostedDocument() || owner != "" {
			continue
		}
		copies[key] = raw
		time.Sleep(time.Millisecond)
	}
	if len(copies) == 0 {
		return nil
	}
	raw, _ := json.Marshal(struct {
		SchemaVersion int               `json:"schemaVersion"`
		Format        string            `json:"cashfluxRecoveryCopies"`
		Copies        map[string]string `json:"copies"`
	}{store.SchemaVersion + 1, "v1", copies})
	return raw
}
