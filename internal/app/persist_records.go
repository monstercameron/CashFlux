// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

const datasetHeadKey = "cashflux:dataset:head"
const datasetRecordPrefix = "cashflux:dataset:record:"

// A head authenticates the exact checkpoint and workspace to which its immutable
// chain belongs. Each transition is a separate ENC3 record. A head and generation
// advance in the SAME strict transaction as those records; the CAS rejects stale
// tabs. An interrupted or failed commit cannot publish a partially written chain.
type datasetRecordHead struct {
	Version    int    `json:"version"`
	Workspace  string `json:"workspace"`
	BaseHash   string `json:"baseHash"`
	Generation string `json:"generation"`
	Tail       string `json:"tail"`
}

type datasetRecordNode struct {
	Parent string   `json:"parent,omitempty"`
	Keys   []string `json:"keys"`
}

func recordWorkspace() string {
	r := loadRegistry()
	if r.ActiveID != "" {
		return r.ActiveID
	}
	return defaultWSID
}

func checkpointHash(raw string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) }

// readDatasetRecords validates the complete chain before callers apply ANY row.
// Missing/unreadable records fail closed rather than presenting an older ledger.
func readDatasetRecords(raw, headRaw, generation, workspace string, read func(string) (string, bool, error)) ([]store.RecordChange, error) {
	if headRaw == "" {
		return nil, nil
	}
	var head datasetRecordHead
	if err := json.Unmarshal([]byte(headRaw), &head); err != nil {
		return nil, fmt.Errorf("read dataset change head: %w", err)
	}
	baseHash := checkpointHash(raw)
	if head.Generation != generation {
		// A distinct complete checkpoint supersedes an older chain. A generation
		// bump alone cannot discard journaled writes beneath the SAME checkpoint.
		if head.BaseHash != baseHash {
			return nil, nil
		}
		return nil, errors.New("the saved checkpoint and change journal have inconsistent generations")
	}
	if head.Version != 1 || head.Workspace != workspace {
		return nil, errors.New("saved changes belong to a different workspace or format")
	}
	if head.BaseHash != baseHash {
		return nil, errors.New("saved changes do not match the dataset checkpoint")
	}

	var nodes []datasetRecordNode
	seen := map[string]bool{}
	for key := head.Tail; key != ""; {
		if seen[key] || !strings.HasPrefix(key, recordWorkspacePrefix(workspace)) {
			return nil, errors.New("saved change chain is invalid")
		}
		seen[key] = true
		value, exists, err := read(key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("saved change batch is missing: %s", key)
		}
		var node datasetRecordNode
		if err := json.Unmarshal([]byte(value), &node); err != nil {
			return nil, fmt.Errorf("read saved change batch: %w", err)
		}
		nodes = append(nodes, node)
		key = node.Parent
	}
	var records []store.RecordChange
	for i := len(nodes) - 1; i >= 0; i-- {
		for _, key := range nodes[i].Keys {
			if !strings.HasPrefix(key, recordWorkspacePrefix(workspace)) || seen[key] {
				return nil, errors.New("saved record key is invalid or repeated")
			}
			seen[key] = true
			value, exists, err := read(key)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("saved record is missing: %s", key)
			}
			var record store.RecordChange
			if err := json.Unmarshal([]byte(value), &record); err != nil {
				return nil, fmt.Errorf("read saved record: %w", err)
			}
			records = append(records, record)
		}
	}
	return records, nil
}

func recordWorkspacePrefix(workspace string) string {
	return datasetRecordPrefix + checkpointHash(workspace) + ":"
}

var hydratedRecordApp *appstate.App
var hydratedRecordCursor, hydratedRecordRevision uint64

func replayDurableDatasetRecords(app *appstate.App, checkpoint string) (result error) {
	defer func() {
		if result == nil {
			hydratedRecordApp = app
			hydratedRecordCursor, _ = app.Store().ChangeCursor()
			hydratedRecordRevision = app.Rev()
		}
	}()
	head, _, err := browserstore.GetSecure(datasetHeadKey)
	if err != nil {
		return err
	}
	if head == "" {
		return nil
	}
	records, err := readDatasetRecords(checkpoint, head, readDatasetGen(), recordWorkspace(), browserstore.GetSecure)
	if err != nil {
		return err
	}
	if err := app.Store().ReplayChanges(records); err != nil {
		return err
	}
	if len(records) > 0 {
		app.SyncTxnLinkNetting()
	}
	return nil
}

type durableWaiter struct {
	revision uint64
	cursor   uint64
	done     func(appstate.DurableResult)
}

func newRecordAutosave(app *appstate.App, commit func(map[string]string, map[string]string, func(error)), push func([]byte, time.Time)) (func(), func()) {
	durableNoticeApp = app
	workspace := recordWorkspace()
	checkpoint := browserstore.GetString(datasetStoreKey)
	var head datasetRecordHead
	_ = json.Unmarshal([]byte(browserstore.GetString(datasetHeadKey)), &head)
	if head.Generation != readDatasetGen() || head.BaseHash != checkpointHash(checkpoint) || head.Workspace != recordWorkspace() {
		head = datasetRecordHead{}
	}
	baseline, baselineErr := localDatasetExport(app)
	mirror, mirrorErr := undosnap.ToSnapshot(baseline)
	baseHash := checkpointHash(checkpoint)
	savedCursor, err := app.Store().ChangeCursor()
	if err != nil {
		app.Log().Error("initialize change persistence", "err", err)
	}
	savedRevision := hydratedRecordRevision
	_, recordCheckpoint, _ := checkpointDatasetJSON([]byte(checkpoint))
	initialized := hadLocalDataset && hydratedRecordApp == app && recordCheckpoint
	if initialized {
		savedCursor = hydratedRecordCursor
	}
	forceVersion, savedForce := uint64(0), uint64(0)
	inFlight := false
	var waiters []durableWaiter
	datasetMyGen = readDatasetGen()
	var save func()
	completeWaiters := func(revision, cursor uint64, err error) {
		if appstate.Default == app && recordWorkspace() == workspace {
			if err != nil {
				uistate.ReportPersistFailure("dataset", func() { save() })
			} else {
				uistate.ClearPersistFailure("dataset")
			}
		}
		pending := waiters
		waiters = nil
		for _, waiter := range pending {
			if (waiter.revision <= revision && waiter.cursor <= cursor) || err != nil {
				waiter.done(appstate.DurableResult{Revision: revision, Err: err})
			} else {
				waiters = append(waiters, waiter)
			}
		}
	}
	save = func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				inFlight = false
				app.Log().Error("save could not finish", "err", recovered)
				completeWaiters(app.Rev(), 0, fmt.Errorf("protected saving could not finish: %v", recovered))
			}
		}()
		if inFlight {
			return
		}
		if appstate.Default != app || recordWorkspace() != workspace {
			completeWaiters(app.Rev(), 0, errors.New("the workspace changed before saving could finish"))
			return
		}
		if readDatasetGen() != datasetMyGen {
			completeWaiters(app.Rev(), 0, errors.New("another tab saved this workspace; reload before saving more changes"))
			return
		}
		if baselineErr != nil || mirrorErr != nil {
			completeWaiters(app.Rev(), 0, errors.New("the saved dataset could not be prepared"))
			return
		}
		if suspendAutosave || hostedHydrationRequired() || pendingEnvelopeRaw != "" || autosaveBlocked() {
			completeWaiters(app.Rev(), 0, errors.New("saving is paused; keep this page open until recovery finishes"))
			return
		}
		if observed := browserstore.GetString(datasetStoreKey); observed != checkpoint {
			// An authorized full replacement (for example a server pull) adopts a
			// new generation outside this closure. Its next save must establish a
			// fresh checkpoint, never append an old chain beneath the new base.
			checkpoint, baseHash, head, initialized = observed, checkpointHash(observed), datasetRecordHead{}, false
		}
		if initialized && savedRevision == app.Rev() && forceVersion == savedForce {
			cursor, err := app.Store().ChangeCursor()
			if err != nil {
				completeWaiters(app.Rev(), 0, err)
				return
			}
			if cursor == savedCursor {
				completeWaiters(savedRevision, savedCursor, nil)
				return
			}
		}
		if recordWorkspace() != workspace || readDatasetGen() != datasetMyGen {
			completeWaiters(app.Rev(), 0, errors.New("another tab saved this workspace; reload before saving more changes"))
			return
		}
		inFlight = true
		// Starting a goroutine alone does not yield the browser. A timer boundary
		// lets pending-state paint precede capture, encryption and IndexedDB work.
		time.AfterFunc(time.Millisecond, func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					inFlight = false
					app.Log().Error("save could not finish", "err", recovered)
					completeWaiters(app.Rev(), 0, fmt.Errorf("protected saving could not finish: %v", recovered))
				}
			}()
			if appstate.Default != app || recordWorkspace() != workspace {
				inFlight = false
				completeWaiters(app.Rev(), 0, errors.New("the workspace changed before saving could finish"))
				return
			}
			captureUndoPoint()
			targetCursor, err := app.Store().ChangeCursor()
			targetRevision, targetForce := app.Rev(), forceVersion
			fail := func(err error) {
				inFlight = false
				app.Log().Error("changes remain unsaved", "err", err)
				completeWaiters(targetRevision, targetCursor, err)
			}
			if err != nil {
				fail(err)
				return
			}
			if initialized && targetCursor == savedCursor && targetForce == savedForce {
				savedRevision = targetRevision
				inFlight = false
				completeWaiters(targetRevision, targetCursor, nil)
				return
			}
			values := map[string]string{}
			var captured []store.RecordChange
			var fullDataset []byte
			nextGeneration := id.New()
			nextHead := head
			full := !initialized || forceVersion != savedForce
			if full {
				data, err := localDatasetExport(app)
				if err != nil {
					fail(err)
					return
				}
				fullDataset = data
				values[datasetStoreKey], values[datasetHeadKey] = encodeRecordCheckpoint(data), ""
				nextHead = datasetRecordHead{}
			} else {
				prefix := recordWorkspacePrefix(workspace) + nextGeneration + ":"
				node := datasetRecordNode{Parent: head.Tail}
				for from := savedCursor; from < targetCursor; {
					page, err := app.Store().CaptureChanges(from, targetCursor, 64)
					if err != nil {
						fail(err)
						return
					}
					captured = append(captured, page.Changes...)

					from = page.Through
					if from < targetCursor {
						time.Sleep(time.Millisecond)
					}
				}

				captured = store.CoalesceChanges(captured)
				for _, record := range captured {
					key := fmt.Sprintf("%s%d", prefix, len(node.Keys))
					encoded, err := json.Marshal(record)
					if err != nil {
						fail(err)
						return
					}
					values[key] = string(encoded)
					node.Keys = append(node.Keys, key)
					if len(node.Keys)%32 == 0 {
						time.Sleep(time.Millisecond)
					}
				}
				encoded, err := json.Marshal(node)
				if err != nil {
					fail(err)
					return
				}
				values[prefix+"batch"] = string(encoded)
				nextHead = datasetRecordHead{Version: 1, Workspace: workspace, BaseHash: baseHash, Generation: nextGeneration, Tail: prefix + "batch"}
				encoded, err = json.Marshal(nextHead)
				if err != nil {
					fail(err)
					return
				}
				values[datasetHeadKey] = string(encoded)
			}
			values[datasetGenKey] = nextGeneration
			if suspendAutosave || appstate.Default != app || recordWorkspace() != workspace || readDatasetGen() != datasetMyGen {
				fail(errors.New("the workspace changed before saving could finish"))
				return
			}

			if !full && len(captured) == 0 {
				savedCursor, savedRevision = targetCursor, targetRevision
				publishDurableUndo(targetCursor)
				inFlight = false
				completeWaiters(targetRevision, targetCursor, nil)
				if app.Rev() != savedRevision || forceVersion != savedForce || len(waiters) > 0 {
					save()
				}
				return
			}
			if persistPerfObserve != nil {
				persistPerfObserve("durable-write")
			}
			expected := map[string]string{datasetGenKey: datasetMyGen}
			if full {
				expected[datasetStoreKey] = checkpoint
			}
			commit(values, expected, func(err error) {
				if err != nil {
					fail(err)
					return
				}
				if full {
					checkpoint = values[datasetStoreKey]
					baseHash = checkpointHash(checkpoint)
					mirror, mirrorErr = undosnap.ToSnapshot(fullDataset)
				} else {
					_, mirrorErr = applyCapturedRecords(mirror, captured)
				}
				head, savedCursor, savedRevision, savedForce = nextHead, targetCursor, targetRevision, targetForce
				initialized, hadLocalDataset, datasetMyGen = true, true, nextGeneration
				publishDurableUndo(targetCursor)
				inFlight = false
				if pruneThrough := min(undoCaptureCursor, targetCursor); pruneThrough > 0 {
					if err := app.Store().PruneChanges(pruneThrough); err != nil {
						app.Log().Warn("release captured changes", "err", err)
					}
				}
				completeWaiters(targetRevision, targetCursor, nil)
				// The legacy sync protocol still needs a portable full JSON snapshot.
				// Assemble from committed records, never from newer live SQL rows.
				if push != nil {
					scheduleCommittedDatasetPush(mirror, nextGeneration, push)
				}
				if app.Rev() != savedRevision || forceVersion != savedForce || len(waiters) > 0 {
					save()
				}
			})
		})
	}
	app.SetDurableWriter(func(revision uint64, done func(appstate.DurableResult)) {
		cursor, err := app.Store().ChangeCursor()
		if err != nil {
			uistate.ReportPersistFailure("dataset", func() { save() })
			done(appstate.DurableResult{Revision: revision, Err: err})
			return
		}
		waiters = append(waiters, durableWaiter{revision: revision, cursor: cursor, done: done})
		save()
	})
	return save, func() { forceVersion++; save() }
}

var committedPushJob uint64

func scheduleCommittedDatasetPush(snapshot history.Snapshot, generation string, push func([]byte, time.Time)) {
	committedPushJob++
	job := committedPushJob
	workspace := recordWorkspace()
	owner, server := signedInUserID(), currentDatasetServerID()
	current := func() bool {
		return job == committedPushJob && !suspendAutosave && recordWorkspace() == workspace && datasetMyGen == generation && signedInUserID() == owner && currentDatasetServerID() == server
	}
	time.AfterFunc(10*time.Millisecond, func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				if app := appstate.Default; app != nil {
					app.Log().Warn("committed sync snapshot could not be prepared", "err", recovered)
				}
			}
		}()
		if !current() {
			return
		}
		data, err := encodeSnapshotSliced(snapshot, func() bool {
			time.Sleep(time.Millisecond)
			return current()
		})
		if err != nil || !current() {
			return
		}
		push(data, time.Now().UTC())
	})
}

// encodeSnapshotSliced assembles the legacy portable format without a SQLite
// export, whole-ledger decode, or one monolithic JSON encoder call. Every 32
// rows return control to the browser. Cancellation prevents mixed revisions.
func encodeSnapshotSliced(snapshot history.Snapshot, yield func() bool) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	collections := make([]string, 0, len(snapshot))
	for collection := range snapshot {
		collections = append(collections, collection)
	}
	sort.Strings(collections)
	written, work := 0, 0
	for _, collection := range collections {
		name := strings.TrimPrefix(collection, "_meta:")
		if written > 0 {
			out.WriteByte(',')
		}
		key, _ := json.Marshal(name)
		out.Write(key)
		out.WriteByte(':')
		rows := snapshot[collection]
		if strings.HasPrefix(collection, "_meta:") {
			value := rows[name]
			if value == nil {
				out.WriteString("null")
			} else {
				out.Write(value)
			}
		} else {
			ids := make([]string, 0, len(rows))
			for id := range rows {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			out.WriteByte('[')
			for i, id := range ids {
				if i > 0 {
					out.WriteByte(',')
				}
				out.Write(rows[id])
				work++
				if work%32 == 0 && !yield() {
					return nil, errors.New("snapshot publication superseded")
				}
			}
			out.WriteByte(']')
		}
		written++
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

// Portable workspace copies remain full datasets, including their committed
// journal. They must not copy only the older checkpoint beneath that journal.
func portableDatasetForBundle(checkpoint string) (string, error) {
	if app := appstate.Default; app != nil && hydrationSucceeded && pendingEnvelopeRaw == "" {
		data, err := app.ExportJSONRedacted()
		return string(data), err
	}
	head, _, err := browserstore.GetSecure(datasetHeadKey)
	if err != nil {
		return "", err
	}
	records, err := readDatasetRecords(checkpoint, head, readDatasetGen(), recordWorkspace(), browserstore.GetSecure)
	if err != nil {
		return "", err
	}
	return portableDatasetFromRecords(checkpoint, records)
}

func portableDatasetFromRecords(checkpoint string, records []store.RecordChange) (string, error) {
	dataset, _, err := checkpointDatasetJSON([]byte(checkpoint))
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return string(dataset), nil
	}
	ds, err := store.Import(dataset)
	if err != nil {
		return "", err
	}
	st, err := store.NewMemory()
	if err != nil {
		return "", err
	}
	defer st.Close()
	if err := st.Load(ds); err != nil {
		return "", err
	}
	if err := st.ReplayChanges(records); err != nil {
		return "", err
	}
	ds, err = st.Snapshot()
	if err != nil {
		return "", err
	}
	data, err := store.Export(ds)
	return string(data), err
}
