// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/store"
)

func setupDatasetSnapshotTest(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	oldMemo, oldExportMemo, oldSnapshotter := datasetUndoSnapshotMemo, datasetExportMemo, snapshotDatasetForUndo
	datasetUndoSnapshotMemo = datasetSnapshotMemo{}
	datasetExportMemo = struct {
		app    *appstate.App
		rev    uint64
		cursor uint64
		data   []byte
	}{}
	snapshotDatasetForUndo = func(app *appstate.App) (store.Dataset, error) { return app.Store().Snapshot() }
	t.Cleanup(func() {
		datasetUndoSnapshotMemo, datasetExportMemo, snapshotDatasetForUndo = oldMemo, oldExportMemo, oldSnapshotter
		_ = app.Store().Close()
	})
	return app
}

func TestSharedDatasetSnapshotCachesDeepCopiesAndInvalidatesOnMutation(t *testing.T) {
	app := setupDatasetSnapshotTest(t)
	if err := app.SetKV("before", `{"v":1}`); err != nil {
		t.Fatal(err)
	}
	captures := 0
	snapshotDatasetForUndo = func(app *appstate.App) (store.Dataset, error) {
		captures++
		return app.Store().Snapshot()
	}

	first, err := sharedDatasetSnapshot(app)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	firstRaw := first["_meta:appState"]["appState"]
	if len(firstRaw) == 0 {
		t.Fatal("missing appState metadata snapshot")
	}
	firstRaw[0] = 'x'
	delete(first["_meta:appState"], "appState")
	delete(first, "_meta:accounts")

	second, err := sharedDatasetSnapshot(app)
	if err != nil {
		t.Fatalf("cached snapshot: %v", err)
	}
	if captures != 1 {
		t.Fatalf("store snapshots = %d, want 1 for unchanged app", captures)
	}
	secondRaw := second["_meta:appState"]["appState"]
	if len(secondRaw) == 0 || bytes.Equal(secondRaw, firstRaw) {
		t.Fatalf("caller mutation reached cached raw bytes: %s", secondRaw)
	}
	if _, ok := second["_meta:accounts"]; !ok {
		t.Fatal("caller deletion reached cached outer snapshot map")
	}
	var state map[string]string
	if err := json.Unmarshal(secondRaw, &state); err != nil || state["before"] != `{"v":1}` {
		t.Fatalf("cached appState = %s, decode err = %v", secondRaw, err)
	}

	oldRevision, oldCursor := app.Rev(), datasetUndoSnapshotMemo.cursor
	if err := app.SetKV("after", "2"); err != nil {
		t.Fatal(err)
	}
	if app.Rev() == oldRevision {
		t.Fatal("test mutation did not advance app revision")
	}
	third, err := sharedDatasetSnapshot(app)
	if err != nil {
		t.Fatalf("snapshot after mutation: %v", err)
	}
	if captures != 2 {
		t.Fatalf("store snapshots after mutation = %d, want 2", captures)
	}
	if datasetUndoSnapshotMemo.cursor == oldCursor {
		t.Fatal("cache cursor did not advance after store mutation")
	}
	var updated map[string]string
	if err := json.Unmarshal(third["_meta:appState"]["appState"], &updated); err != nil || updated["after"] != "2" {
		t.Fatalf("updated appState = %#v, decode err = %v", updated, err)
	}
}

func TestSharedDatasetSnapshotRetriesMutationDuringCapture(t *testing.T) {
	app := setupDatasetSnapshotTest(t)
	captures := 0
	snapshotDatasetForUndo = func(app *appstate.App) (store.Dataset, error) {
		captures++
		dataset, err := app.Store().Snapshot()
		if captures == 1 && err == nil {
			err = app.SetKV("during-capture", "yes")
		}
		return dataset, err
	}

	snapshot, err := sharedDatasetSnapshot(app)
	if err != nil {
		t.Fatalf("snapshot after bounded retry: %v", err)
	}
	if captures != 2 {
		t.Fatalf("store snapshots = %d, want one stale attempt and one retry", captures)
	}
	var state map[string]string
	if err := json.Unmarshal(snapshot["_meta:appState"]["appState"], &state); err != nil || state["during-capture"] != "yes" {
		t.Fatalf("returned snapshot did not include concurrent store mutation: %#v, err=%v", state, err)
	}
}

func TestSharedDatasetSnapshotDoesNotPublishAfterRepeatedMutation(t *testing.T) {
	app := setupDatasetSnapshotTest(t)
	captures := 0
	snapshotDatasetForUndo = func(app *appstate.App) (store.Dataset, error) {
		captures++
		dataset, err := app.Store().Snapshot()
		if err == nil {
			err = app.SetKV("change", string(rune('0'+captures)))
		}
		return dataset, err
	}

	if snapshot, err := sharedDatasetSnapshot(app); err == nil || snapshot != nil {
		t.Fatalf("repeated concurrent mutation returned snapshot=%v err=%v; want bounded error", snapshot, err)
	}
	if captures != 2 {
		t.Fatalf("store snapshots = %d, want bounded retry count 2", captures)
	}
	if datasetUndoSnapshotMemo.snapshot != nil {
		t.Fatal("published a stale snapshot under a newer app key")
	}
}

func TestSharedDatasetSnapshotReusesCurrentExportMemo(t *testing.T) {
	app := setupDatasetSnapshotTest(t)
	data, err := app.ExportJSONRedacted()
	if err != nil {
		t.Fatalf("ExportJSONRedacted: %v", err)
	}
	cursor, err := app.Store().ChangeCursor()
	if err != nil {
		t.Fatal(err)
	}
	datasetExportMemo = struct {
		app    *appstate.App
		rev    uint64
		cursor uint64
		data   []byte
	}{app, app.Rev(), cursor, data}
	snapshotDatasetForUndo = func(*appstate.App) (store.Dataset, error) {
		t.Fatal("matching export memo should avoid another store snapshot")
		return store.Dataset{}, nil
	}

	if _, err := sharedDatasetSnapshot(app); err != nil {
		t.Fatalf("snapshot from export memo: %v", err)
	}
}

func TestCurrentSnapshotAndWriterBaselineSharePreparedSnapshot(t *testing.T) {
	app := setupDatasetSnapshotTest(t)
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })
	captures := 0
	snapshotDatasetForUndo = func(app *appstate.App) (store.Dataset, error) {
		captures++
		return app.Store().Snapshot()
	}

	if _, err := currentSnapshot(); err != nil {
		t.Fatalf("currentSnapshot: %v", err)
	}
	if _, err := sharedDatasetSnapshot(app); err != nil {
		t.Fatalf("record-writer baseline snapshot: %v", err)
	}
	if captures != 1 {
		t.Fatalf("store snapshots across undo init and writer baseline = %d, want 1", captures)
	}
}

func TestSharedDatasetSnapshotReturnsCursorStorageErrors(t *testing.T) {
	app := setupDatasetSnapshotTest(t)
	if err := app.Store().Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	if snapshot, err := sharedDatasetSnapshot(app); err == nil || snapshot != nil || errors.Is(err, errDatasetSnapshotChanged) {
		t.Fatalf("closed store result = (%v, %v), want underlying cursor error", snapshot, err)
	}
}
