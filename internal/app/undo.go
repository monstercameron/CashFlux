// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

// undoStack is the bounded per-session undo/redo stack.  4 MB gives roughly
// 50–200 undo steps for a typical household dataset.
var undoStack = history.NewStack(4 * 1024 * 1024)

// lastUndoSnap is the snapshot that corresponds to the current (post-stack)
// state of the dataset.  It is updated on every captureUndoPoint call and on
// every undo/redo application so the stack cursor always reflects reality.
var lastUndoSnap history.Snapshot

// applyingUndo is true while undoLastChange / redoLastChange are writing back
// to the dataset, preventing captureUndoPoint (triggered by the autosave) from
// recording the restoration itself as a new undo step.
var applyingUndo bool
var undoPersistencePending bool
var undoDurableCursor uint64

var lastUndoCaptureApp *appstate.App
var lastUndoCaptureRev uint64

// initUndo captures the baseline snapshot immediately after the dataset has
// been hydrated from localStorage (or seeded).  Call once, after hydrateDataset.
func initUndo() {
	snap, err := currentSnapshot()
	if err != nil {
		appstate.Default.Log().Error("initUndo: snapshot failed", "err", err)
		return
	}
	lastUndoSnap = snap
	undoCaptureCursor, _ = appstate.Default.Store().ChangeCursor()
	undoPending = nil
	lastUndoCaptureApp, lastUndoCaptureRev = appstate.Default, appstate.Default.Rev()
}

// captureUndoPoint prepares history from changed records only. It updates the
// live baseline and stages undo/audit changes; persistence publishes staged undo
// points only after the strict commit succeeds. Replay does not capture itself.
func captureUndoPoint() {
	if applyingUndo {
		return
	}
	if app := appstate.Default; app != nil && lastUndoCaptureApp == app && lastUndoCaptureRev == app.Rev() {
		return
	}
	if app := appstate.Default; app != nil {
		if lastUndoCaptureApp != app || lastUndoSnap == nil {
			initUndo()
			return
		}
		if err := captureIncrementalUndo(app); err != nil {
			app.Log().Error("capture undo changes failed", "err", err)
		}
	}
}

// capturedSkipCollections are dataset collections that mutate as DERIVED UI
// state (not user actions), so they must not become undo steps or activity
// entries. The dashboard writes its placement mirror on every render, and the
// audit log records every capture: without skipping auditEntries here, the
// autosave that follows a captured mutation diffs the just-written audit row
// into an auditEntries-only undo step — the stack's top entry then undoes
// nothing the user can see, so the toast's "Undo" silently no-ops.
var capturedSkipCollections = map[string]bool{
	"transferOperations": true,
	"placements":         true,
	"auditEntries":       true,
}

// derivedKVKeys maps the dataset's scalar KV collections to the keys inside
// them that mutate as DERIVED, self-healing state rather than user actions:
// the notification feed auto-resolves alerts whose condition cleared (e.g.
// right after "Mark all updated" refreshes every stale balance), the engines
// re-persist their delivery log / detection state, and the health score
// records a trend point on every recompute. If such a write lands after a
// user mutation's capture, it becomes the stack's top entry and STEALS the
// undoable toast's target — clicking Undo silently reverts an invisible blob
// instead of the user's change (#77). Same rationale as placements above.
var derivedKVKeys = map[string]map[string]bool{
	"_meta:appState": {
		"cashflux:notify:feed":      true,
		"cashflux:notify:delivered": true,
		"cashflux:health:trend":     true,
	},
	"_meta:settingsState": {
		"cashflux:smart-settings": true,
	},
}

// filterCapturedChanges removes derived-UI-state changes from a change set so
// only genuine user mutations reach the undo stack and the activity feed. A
// change set whose ONLY real content is scalar-KV updates confined to derived
// keys (or no-op re-serializations) is emptied entirely — absorbed into the
// baseline, not undoable, not shown.
func filterCapturedChanges(cs history.ChangeSet) history.ChangeSet {
	kept := cs.Changes[:0:0]
	for _, c := range cs.Changes {
		if capturedSkipCollections[c.Collection] {
			continue
		}
		kept = append(kept, c)
	}
	cs.Changes = kept
	if derivedKVOnly(cs) {
		cs.Changes = nil
	}
	return cs
}

// derivedKVOnly reports whether every change in cs is an update to a scalar KV
// collection whose before/after differ only in that collection's derived keys.
// An update whose diff is EMPTY (a byte-level re-serialization of identical
// state) counts too — undoing it would visibly do nothing.
func derivedKVOnly(cs history.ChangeSet) bool {
	if len(cs.Changes) == 0 {
		return false
	}
	for _, c := range cs.Changes {
		derived, kvColl := derivedKVKeys[c.Collection]
		if !kvColl || c.Op != history.OpUpdate {
			return false
		}
		keys, ok := history.ScalarMapDiffKeys(c.Before, c.After)
		if !ok {
			return false
		}
		for _, k := range keys {
			if !derived[k] {
				return false
			}
		}
	}
	return true
}

// undoLastChange pops the most recent change from the stack, applies its
// inverse to the dataset, and bumps the UI revision so screens re-render.
// Returns true when an undo request was accepted; completion is asynchronous.
func undoLastChange() bool {
	if applyingUndo || (!undoPersistencePending && !undoStack.CanUndo()) {
		return false
	}
	undoLastChangeThen(false, func(err error) {
		if err != nil {
			uistate.PostNotice(uistate.T("bv6d1.undoTheSaveFailed"), true)
		}
	})
	return true
}

func undoLastChangeThen(redo bool, done func(error)) {
	app := appstate.Default
	if app == nil || applyingUndo {
		done(fmt.Errorf("another change is still being saved"))
		return
	}

	if undoPersistencePending {
		app.RequestDurable(func(result appstate.DurableResult) {
			if result.Err == nil {
				undoPersistencePending = false
			}
			done(result.Err)
		})
		return
	}

	captureUndoPoint()
	if len(undoPending) > 0 {
		applyingUndo = true
		app.RequestDurable(func(result appstate.DurableResult) {
			applyingUndo = false
			if result.Err != nil {
				done(result.Err)
				return
			}
			undoLastChangeThen(redo, done)
		})
		return
	}
	var cs history.ChangeSet
	var ok bool
	if redo {
		cs, ok = undoStack.Redo()
	} else {
		cs, ok = undoStack.Undo()
	}
	if !ok {
		done(fmt.Errorf("there is no change to undo"))
		return
	}
	restoreCursor := func() {
		if redo {
			_, _ = undoStack.Undo()
		} else {
			_, _ = undoStack.Redo()
		}
	}
	records, err := undoRecordChanges(cs)
	if err != nil {
		restoreCursor()
		done(err)
		return
	}
	applyingUndo = true
	undoPersistencePending = true
	if err = app.Store().ReplayChangesExpected(records); err != nil {
		applyingUndo = false
		undoPersistencePending = false
		restoreCursor()
		done(err)
		return
	}
	if _, err = applyCapturedRecords(lastUndoSnap, records); err != nil {
		applyingUndo = false
		done(err)
		return
	}
	undoCaptureCursor, _ = app.Store().ChangeCursor()
	undoDurableCursor = undoCaptureCursor
	lastUndoCaptureApp, lastUndoCaptureRev = app, app.Rev()
	uistate.BumpDataRevision()
	app.SyncTxnLinkNetting()
	runNotifyCatchUp()
	ids := []string{}
	for _, record := range records {
		if record.Table == "transactions" {
			ids = append(ids, record.ID)
		}
	}
	app.RequestTransactionsDurable(ids, func(result appstate.DurableResult) {
		applyingUndo = false
		if result.Err == nil {
			undoPersistencePending = false
		}
		// On failure retain the live restoration and dirty capture. A second undo
		// must not move the cursor farther until this restoration has committed.
		done(result.Err)
	})
}

func undoRecordChanges(cs history.ChangeSet) ([]store.RecordChange, error) {
	tables := map[string]string{}
	for table, collection := range recordCollections {
		tables[collection] = table
	}
	var records []store.RecordChange
	for _, c := range cs.Changes {
		if c.Collection == "_meta:settings" {
			records = append(records, store.RecordChange{Table: "settings", ID: "app", Before: c.Before, After: c.After})
			continue
		}
		if c.Collection == "_meta:appState" || c.Collection == "_meta:settingsState" {
			table := "appkv"
			if c.Collection == "_meta:settingsState" {
				table = "settingskv"
			}
			before, after := map[string]string{}, map[string]string{}
			if len(c.Before) > 0 {
				if err := json.Unmarshal(c.Before, &before); err != nil {
					return nil, err
				}
			}
			if len(c.After) > 0 {
				if err := json.Unmarshal(c.After, &after); err != nil {
					return nil, err
				}
			}
			keys := map[string]bool{}
			for key := range before {
				keys[key] = true
			}
			for key := range after {
				keys[key] = true
			}
			for key := range keys {
				b, bok := before[key]
				a, aok := after[key]
				if bok == aok && a == b {
					continue
				}
				record := store.RecordChange{Table: table, ID: key}
				if bok {
					record.Before, _ = json.Marshal(b)
				}
				if aok {
					record.After, _ = json.Marshal(a)
				}
				records = append(records, record)
			}
			continue
		}
		table, ok := tables[c.Collection]
		if !ok {
			return nil, fmt.Errorf("cannot undo collection %q", c.Collection)
		}
		records = append(records, store.RecordChange{Table: table, ID: c.ID, Before: c.Before, After: c.After})
	}
	return records, nil
}

// currentSnapshot converts the shared credential-free export into an undo
// snapshot. Device credentials are outside dataset state and never enter undo.
func currentSnapshot() (history.Snapshot, error) {
	data, err := localDatasetExport(appstate.Default)
	if err != nil {
		return nil, err
	}
	return undosnap.ToSnapshot(data)
}

func requestUndoNotice(redo bool) {
	available := undoStack.CanUndo()
	emptyKey, successKey := "cmd.nothingToUndo", "cmd.undone"
	if redo {
		available = undoStack.CanRedo()
		emptyKey, successKey = "cmd.nothingToRedo", "cmd.redone"
	}
	if !available && !undoPersistencePending && len(undoPending) == 0 {
		paletteNotify(uistate.T(emptyKey), false)
		return
	}
	paletteNotify("Saving undo...", false)
	undoLastChangeThen(redo, func(err error) {
		if err != nil {
			paletteNotify("This undo is not saved. Keep this page open and try again.", true)
			return
		}
		paletteNotify(uistate.T(successKey), false)
	})
}
