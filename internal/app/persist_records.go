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
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

const datasetHeadKey = "cashflux:dataset:head"
const datasetRecordPrefix = "cashflux:dataset:record:"

const journalCheckpointRecordLimit = 128

func shouldCheckpointJournal(initialized, forced bool, journalRecords int) bool {
	return !initialized || forced || journalRecords >= journalCheckpointRecordLimit
}

func datasetSaveGenerationError(workspaceMatches, generationMatches bool) error {
	if workspaceMatches && !generationMatches {
		return appstate.ErrStaleDataset
	}
	return errors.New("the workspace changed before saving could finish")
}

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
	Cursor     uint64 `json:"cursor,omitempty"`
}

type datasetRecordNode struct {
	Parent           string   `json:"parent,omitempty"`
	Keys             []string `json:"keys"`
	Generation       string   `json:"generation,omitempty"`
	ParentGeneration string   `json:"parentGeneration,omitempty"`
	BaseHash         string   `json:"baseHash,omitempty"`
	Workspace        string   `json:"workspace,omitempty"`
	From             uint64   `json:"from,omitempty"`
	Through          uint64   `json:"through,omitempty"`
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
	nodeGeneration, nodeCursor := head.Generation, head.Cursor
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
		if node.Generation != "" {
			if node.Generation != nodeGeneration || node.Workspace != workspace || node.BaseHash != baseHash || node.Through != nodeCursor || node.Through != node.From+1 {
				return nil, errors.New("saved change generations or cursors do not chain")
			}
			nodeGeneration, nodeCursor = node.ParentGeneration, node.From
		}
		nodes = append(nodes, node)
		key = node.Parent
	}
	if generation := checkpointGeneration(raw); generation != "" && (nodeGeneration != generation || nodeCursor != 0) {
		return nil, errors.New("saved changes do not chain from the checkpoint generation")
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

func journalKeysFromHead(head datasetRecordHead, workspace string, read func(string) (string, bool, error)) ([]string, error) {
	var keys []string
	seen := map[string]bool{}
	prefix := recordWorkspacePrefix(workspace)
	for key := head.Tail; key != ""; {
		if seen[key] || !strings.HasPrefix(key, prefix) {
			return nil, errors.New("saved change chain is invalid")
		}
		seen[key] = true
		raw, exists, err := read(key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("saved change batch is missing: %s", key)
		}
		var node datasetRecordNode
		if err := json.Unmarshal([]byte(raw), &node); err != nil {
			return nil, fmt.Errorf("read saved change batch: %w", err)
		}
		for _, recordKey := range node.Keys {
			if !strings.HasPrefix(recordKey, prefix) || seen[recordKey] {
				return nil, errors.New("saved journal record key is invalid or repeated")
			}
			seen[recordKey] = true
			keys = append(keys, recordKey)
		}
		keys = append(keys, key)
		key = node.Parent
	}
	return keys, nil
}

var hydratedRecordApp *appstate.App
var hydratedRecordCursor, hydratedRecordRevision uint64

func replayDurableDatasetRecords(app *appstate.App, _ string) error {
	snapshot, err := readDurableDatasetSnapshot()
	if err != nil {
		return err
	}
	return hydrateDurableDataset(app, snapshot)
}

type durableWaiter struct {
	revision uint64
	cursor   uint64
	done     func(appstate.DurableResult)
}

func newRecordAutosave(app *appstate.App, commit func(map[string]string, map[string]string, func(error)), push func([]byte, time.Time)) (func(), func()) {
	resetStaleDatasetState()
	durableNoticeApp = app
	workspace := recordWorkspace()
	checkpoint := browserstore.GetString(datasetStoreKey)
	writerGeneration := readDatasetGen()
	writerOwner := browserstore.GetString(datasetOwnerKey)
	headRaw := browserstore.GetString(datasetHeadKey)
	if hydratedDatasetState.app == app {
		checkpoint = hydratedDatasetState.snapshot.Values[datasetStoreKey]
		writerGeneration = hydratedDatasetState.snapshot.Values[datasetGenKey]
		writerOwner = hydratedDatasetState.snapshot.Values[datasetOwnerKey]
		headRaw = hydratedDatasetState.snapshot.Values[datasetHeadKey]
	}
	var head datasetRecordHead
	_ = json.Unmarshal([]byte(headRaw), &head)
	if head.Generation != writerGeneration || head.BaseHash != checkpointHash(checkpoint) || head.Workspace != workspace {
		head = datasetRecordHead{}
	}
	readJournal := browserstore.GetSecure
	if hydratedDatasetState.app == app {
		readJournal = hydratedDatasetState.snapshot.Read
	}
	journalKeys, journalErr := journalKeysFromHead(head, workspace, readJournal)
	if journalErr != nil {
		app.Log().Error("load journal compaction state", "err", journalErr)
		journalKeys = nil
	}
	mirror, snapshotErr := sharedDatasetSnapshot(app)
	var mirrorErr error
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
	datasetMyGen = writerGeneration
	paused := func() bool {
		return suspendAutosave || syncApplyPaused || hostedHydrationRequired() || pendingEnvelopeRaw != "" || autosaveBlocked()
	}
	pausedErr := errors.New("saving is paused; keep this page open until recovery finishes")
	var save func()
	completeWaiters := func(revision, cursor uint64, err error) {
		if appstate.Default == app && recordWorkspace() == workspace {
			if err != nil {
				if errors.Is(err, appstate.ErrStaleDataset) {
					uistate.ClearPersistFailure("dataset")
					preserveStaleDatasetEdits(app, workspace, writerOwner)
				} else {
					uistate.ReportPersistFailure("dataset", func() { save() })
				}
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
		if staleDatasetBlocked {
			completeWaiters(app.Rev(), 0, appstate.ErrStaleDataset)
			return
		}
		if appstate.Default != app || recordWorkspace() != workspace {
			completeWaiters(app.Rev(), 0, errors.New("the workspace changed before saving could finish"))
			return
		}
		// A durable replacement publishes its cache before its completion
		// goroutine adopts the generation. While it owns the pause, that window
		// must not be mistaken for a foreign writer and made permanently stale.
		if paused() {
			completeWaiters(app.Rev(), 0, pausedErr)
			return
		}
		// Only an explicit full replacement adopted by this hydrated App may
		// rebase its writer. Another App's generation is never our entitlement.
		observed := browserstore.GetString(datasetStoreKey)
		if lastAcceptedOwnerClaim.app == app && lastAcceptedOwnerClaim.workspace == workspace && browserstore.GetString(datasetOwnerKey) == lastAcceptedOwnerClaim.value {
			writerOwner = lastAcceptedOwnerClaim.value
		}
		if datasetMyGen != writerGeneration && hydratedRecordApp == app && datasetMyGen == readDatasetGen() {
			writerGeneration = datasetMyGen
			checkpoint, baseHash, head, headRaw, initialized = observed, checkpointHash(observed), datasetRecordHead{}, "", false
		}
		if readDatasetGen() != writerGeneration || observed != checkpoint || browserstore.GetString(datasetHeadKey) != headRaw {
			completeWaiters(app.Rev(), 0, appstate.ErrStaleDataset)
			return
		}
		if snapshotErr != nil || mirrorErr != nil {
			completeWaiters(app.Rev(), 0, errors.New("the saved dataset could not be prepared"))
			return
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
		if recordWorkspace() != workspace || readDatasetGen() != writerGeneration {
			completeWaiters(app.Rev(), 0, datasetSaveGenerationError(recordWorkspace() == workspace, readDatasetGen() == writerGeneration))
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
			if paused() {
				inFlight = false
				completeWaiters(app.Rev(), 0, pausedErr)
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
			expectedGeneration := writerGeneration
			nextHead := head
			full := shouldCheckpointJournal(initialized, forceVersion != savedForce, len(journalKeys))
			if full {
				data, err := localDatasetExport(app)
				if err != nil {
					fail(err)
					return
				}
				fullDataset = data
				values[datasetStoreKey], values[datasetHeadKey] = encodeRecordCheckpointGeneration(data, nextGeneration), ""
				values[datasetCheckpointBackupKey] = encodeCheckpointBackup(workspace, values[datasetStoreKey], writerOwner)
				for _, key := range journalKeys {
					values[key] = ""
				}
				nextHead = datasetRecordHead{}
			} else {
				prefix := recordWorkspacePrefix(workspace) + nextGeneration + ":"
				node := datasetRecordNode{Parent: head.Tail, Generation: nextGeneration, ParentGeneration: expectedGeneration, BaseHash: baseHash, Workspace: workspace, From: head.Cursor, Through: head.Cursor + 1}
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
				nextHead = datasetRecordHead{Version: 1, Workspace: workspace, BaseHash: baseHash, Generation: nextGeneration, Tail: prefix + "batch", Cursor: node.Through}
				encoded, err = json.Marshal(nextHead)
				if err != nil {
					fail(err)
					return
				}
				values[datasetHeadKey] = string(encoded)
			}
			values[datasetGenKey] = nextGeneration
			if paused() {
				fail(pausedErr)
				return
			}
			if appstate.Default != app || recordWorkspace() != workspace || readDatasetGen() != expectedGeneration {
				fail(datasetSaveGenerationError(recordWorkspace() == workspace, readDatasetGen() == expectedGeneration))
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
			expected := map[string]string{datasetGenKey: expectedGeneration, datasetStoreKey: checkpoint, datasetHeadKey: headRaw, datasetOwnerKey: writerOwner}
			commit(values, expected, func(err error) {
				if err != nil {
					if errors.Is(err, atrest.ErrRecordChanged) {
						err = fmt.Errorf("saved data changed in another instance: %w", appstate.ErrStaleDataset)
					}
					fail(err)
					return
				}
				if appstate.Default != app || recordWorkspace() != workspace {
					fail(errors.New("the workspace changed before saving could finish"))
					return
				}
				if full {
					checkpoint = values[datasetStoreKey]
					baseHash = checkpointHash(checkpoint)
					journalKeys = nil
					mirror, mirrorErr = undosnap.ToSnapshot(fullDataset)
				} else {
					for key := range values {
						if strings.HasPrefix(key, recordWorkspacePrefix(workspace)) {
							journalKeys = append(journalKeys, key)
						}
					}
					_, mirrorErr = applyCapturedRecords(mirror, captured)
				}
				head, savedCursor, savedRevision, savedForce = nextHead, targetCursor, targetRevision, targetForce
				headRaw, writerGeneration = values[datasetHeadKey], nextGeneration
				initialized, hadLocalDataset, datasetMyGen = true, true, nextGeneration
				// A fresh or intentionally empty household gains the same local
				// writer authority as an imported checkpoint only after its first
				// save commits. App Lock can then adopt its own replacement without
				// mistaking it for another page's write.
				hydratedRecordApp = app
				hydratedRecordCursor, hydratedRecordRevision = targetCursor, targetRevision
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
		return job == committedPushJob && !suspendAutosave && !syncApplyPaused && recordWorkspace() == workspace && datasetMyGen == generation && signedInUserID() == owner && currentDatasetServerID() == server
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
	snapshot, err := readDurableDatasetSnapshot()
	if err != nil {
		return "", err
	}
	checkpoint = snapshot.Values[datasetStoreKey]
	head, _, err := snapshot.Read(datasetHeadKey)
	if err != nil {
		return "", err
	}
	records, err := readDatasetRecords(checkpoint, head, snapshot.Values[datasetGenKey], snapshot.workspace, snapshot.Read)
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
