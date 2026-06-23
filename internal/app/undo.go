//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/history"
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

// initUndo captures the baseline snapshot immediately after the dataset has
// been hydrated from localStorage (or seeded).  Call once, after hydrateDataset.
func initUndo() {
	snap, err := currentSnapshot()
	if err != nil {
		appstate.Default.Log().Error("initUndo: snapshot failed", "err", err)
		return
	}
	lastUndoSnap = snap
}

// captureUndoPoint diffs the current dataset against the last recorded snapshot
// and pushes a new entry onto the stack when anything changed.  It is called
// from the autosave's save() function whenever the serialized dataset changes,
// so every mutation is automatically captured without instrumenting individual
// write paths.
//
// The call is a no-op while an undo/redo application is in flight (guarded by
// applyingUndo) so the restoration doesn't become its own undo step.
func captureUndoPoint() {
	if applyingUndo {
		return
	}
	snap, err := currentSnapshot()
	if err != nil {
		appstate.Default.Log().Error("captureUndoPoint: snapshot failed", "err", err)
		return
	}
	cs := history.Diff(lastUndoSnap, snap)
	if cs.IsEmpty() {
		return
	}
	undoStack.Push(cs)
	// Feed the audit log so the Activity timeline shows a per-change entry (C78).
	RecordAuditPoint(cs)
	lastUndoSnap = snap
}

// undoLastChange pops the most recent change from the stack, applies its
// inverse to the dataset, and bumps the UI revision so screens re-render.
// Returns true when an undo was performed, false when the stack is empty.
func undoLastChange() bool {
	cs, ok := undoStack.Undo()
	if !ok {
		return false
	}
	// cs from Stack.Undo() is already the inverted change set (moves backward).
	newSnap := cs.Apply(lastUndoSnap)
	return applySnapshot(newSnap)
}

// redoLastChange re-applies the most recently undone change to the dataset and
// bumps the UI revision.  Returns true when a redo was performed.
func redoLastChange() bool {
	cs, ok := undoStack.Redo()
	if !ok {
		return false
	}
	// cs from Stack.Redo() is the forward change set (re-applies the mutation).
	newSnap := cs.Apply(lastUndoSnap)
	return applySnapshot(newSnap)
}

// applySnapshot writes newSnap into the live dataset, updates lastUndoSnap,
// and triggers a UI re-render.  Returns true on success.
func applySnapshot(newSnap history.Snapshot) bool {
	data, err := undosnap.ToJSON(newSnap)
	if err != nil {
		appstate.Default.Log().Error("applySnapshot: ToJSON failed", "err", err)
		// Roll the stack cursor back so the stack stays consistent.
		return false
	}
	applyingUndo = true
	defer func() { applyingUndo = false }()

	if err := appstate.Default.ImportJSON(data); err != nil {
		appstate.Default.Log().Error("applySnapshot: ImportJSON failed", "err", err)
		return false
	}
	lastUndoSnap = newSnap
	uistate.BumpDataRevision() // re-render screens after the dataset is replaced
	return true
}

// currentSnapshot exports the dataset and converts it to a history.Snapshot.
// It uses ExportJSON (non-redacted) so the full round-trip is lossless; the
// OpenAI key is included in undo snapshots but never written to localStorage
// (the autosave calls ExportJSONRedacted separately).
func currentSnapshot() (history.Snapshot, error) {
	data, err := appstate.Default.ExportJSON()
	if err != nil {
		return nil, err
	}
	return undosnap.ToSnapshot(data)
}
