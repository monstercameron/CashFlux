// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

type datasetSnapshotMemo struct {
	app      *appstate.App
	revision uint64
	cursor   uint64
	snapshot history.Snapshot
}

var datasetUndoSnapshotMemo datasetSnapshotMemo

var snapshotDatasetForUndo = func(app *appstate.App) (store.Dataset, error) {
	return app.Store().Snapshot()
}

var errDatasetSnapshotChanged = errors.New("dataset changed while preparing an undo snapshot")

// sharedDatasetSnapshot returns an independent snapshot for the current app
// revision and store cursor. The cache is shared by startup undo initialization
// and the record writer's baseline mirror; callers may mutate their copy.
func sharedDatasetSnapshot(app *appstate.App) (history.Snapshot, error) {
	if app == nil {
		return nil, errors.New("cannot snapshot a nil app")
	}
	for attempt := 0; attempt < 2; attempt++ {
		revision, cursor, err := datasetSnapshotIdentity(app)
		if err != nil {
			if errors.Is(err, errDatasetSnapshotChanged) {
				continue
			}
			return nil, err
		}
		if memo := datasetUndoSnapshotMemo; memo.app == app && memo.revision == revision && memo.cursor == cursor && memo.snapshot != nil {
			if err := verifyDatasetSnapshotIdentity(app, revision, cursor); err != nil {
				if errors.Is(err, errDatasetSnapshotChanged) {
					continue
				}
				return nil, err
			}
			return cloneHistorySnapshot(memo.snapshot), nil
		}

		var snapshot history.Snapshot
		if datasetExportMemo.app == app && datasetExportMemo.rev == revision && datasetExportMemo.cursor == cursor && datasetExportMemo.data != nil {
			snapshot, err = undosnap.ToSnapshot(datasetExportMemo.data)
		} else {
			var dataset store.Dataset
			dataset, err = snapshotDatasetForUndo(app)
			if err == nil {
				snapshot, err = undosnap.ToSnapshotDataset(dataset)
			}
		}
		if err != nil {
			return nil, err
		}
		if err := verifyDatasetSnapshotIdentity(app, revision, cursor); err != nil {
			if errors.Is(err, errDatasetSnapshotChanged) {
				continue
			}
			return nil, err
		}
		datasetUndoSnapshotMemo = datasetSnapshotMemo{app: app, revision: revision, cursor: cursor, snapshot: snapshot}
		return cloneHistorySnapshot(snapshot), nil
	}
	return nil, errDatasetSnapshotChanged
}

func datasetSnapshotIdentity(app *appstate.App) (uint64, uint64, error) {
	revision := app.Rev()
	cursor, err := app.Store().ChangeCursor()
	if err != nil {
		return 0, 0, err
	}
	if app.Rev() != revision {
		return 0, 0, errDatasetSnapshotChanged
	}
	return revision, cursor, nil
}

func verifyDatasetSnapshotIdentity(app *appstate.App, revision, cursor uint64) error {
	if app.Rev() != revision {
		return errDatasetSnapshotChanged
	}
	currentCursor, err := app.Store().ChangeCursor()
	if err != nil {
		return err
	}
	if app.Rev() != revision || currentCursor != cursor {
		return errDatasetSnapshotChanged
	}
	return nil
}

func cloneHistorySnapshot(snapshot history.Snapshot) history.Snapshot {
	if snapshot == nil {
		return nil
	}
	clone := make(history.Snapshot, len(snapshot))
	for collection, rows := range snapshot {
		if rows == nil {
			clone[collection] = nil
			continue
		}
		clonedRows := make(map[string]json.RawMessage, len(rows))
		for key, raw := range rows {
			if raw == nil {
				clonedRows[key] = nil
			} else {
				cloned := make(json.RawMessage, len(raw))
				copy(cloned, raw)
				clonedRows[key] = cloned
			}
		}
		clone[collection] = clonedRows
	}
	return clone
}
