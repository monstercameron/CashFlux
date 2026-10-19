// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type txnSaveStatus struct {
	value           txnSaveState
	atom            state.Atom[txnSaveState]
	mounted         bool
	mountGeneration uint64
	operation       uint64
}

type txnSaveState uint8

const (
	txnSaveIdle txnSaveState = iota
	txnSavePending
	txnSaveFailed
)

var txnSaveStatuses = map[string]txnSaveStatus{}
var txnSaveMountGeneration uint64

type txnFormLease struct {
	id         string
	active     bool
	generation uint64
}

func useTxnFormLease(id string) *txnFormLease {
	lease := ui.UseState(&txnFormLease{id: id, active: true, generation: 1}).Get()
	if lease.id != id {
		lease.id = id
		lease.active = true
		lease.generation++
	}
	ui.UseEffect(func() func() {
		lease.active = true
		generation := lease.generation
		return func() {
			if lease.id == id && lease.generation == generation {
				lease.active = false
				lease.generation++
			}
		}
	}, id)
	return lease
}

func (lease *txnFormLease) check() func() bool {
	generation := lease.generation
	return func() bool { return lease.active && lease.generation == generation }
}

var txnDurabilityApp *appstate.App
var txnDurabilityCancel func()
var txnDurabilityNamespace uint64

func observeTxnDurability(app *appstate.App) {
	if app == nil || txnDurabilityApp == app {
		return
	}
	if txnDurabilityCancel != nil {
		txnDurabilityCancel()
	}
	txnDurabilityApp = app
	txnDurabilityNamespace++
	txnSaveStatuses = map[string]txnSaveStatus{}
	txnDurabilityCancel = app.OnTransactionDurability(func(event appstate.TransactionDurability) {
		for _, id := range event.TransactionIDs {
			status := txnSaveStatuses[id]
			if event.Pending {
				status.operation = event.Operation
				txnSaveStatuses[id] = status
				setTxnSaveStatus(id, txnSavePending)
				continue
			}
			if status.operation != event.Operation {
				continue
			}
			value := txnSaveIdle
			if event.Result.Err != nil {
				value = txnSaveFailed
			}
			setTxnSaveStatus(id, value)
		}
	})
}

func useTxnSaveStatus(id string) string {
	observeTxnDurability(appstate.Default)
	status := txnSaveStatuses[id]
	atom := state.UseAtom(fmt.Sprintf("transaction:durable:%d:%s", txnDurabilityNamespace, id), status.value)
	mountGeneration := ui.UseMemo(func() uint64 { txnSaveMountGeneration++; return txnSaveMountGeneration }, txnDurabilityNamespace, id)
	status.atom, status.mounted, status.mountGeneration = atom, true, mountGeneration
	txnSaveStatuses[id] = status
	ui.UseEffect(func() func() {
		return func() {
			current, ok := txnSaveStatuses[id]
			if !ok || current.mountGeneration != mountGeneration {
				return
			}
			current.mounted = false
			if current.value == txnSavePending {
				txnSaveStatuses[id] = current
				return
			}
			delete(txnSaveStatuses, id)
		}
	}, id)
	return txnSaveStatusText(atom.Get())
}

func txnSaveStatusText(status txnSaveState) string {
	switch status {
	case txnSavePending:
		return uistate.T("tc3w4.saving")
	case txnSaveFailed:
		return uistate.T("tc3w4.notSaved")
	default:
		return ""
	}
}

func setTxnSaveStatus(id string, value txnSaveState) {
	status := txnSaveStatuses[id]
	if status.value == value {
		return
	}
	status.value = value
	txnSaveStatuses[id] = status
	if status.mounted {
		status.atom.Set(value)
	} else if value != txnSavePending {
		delete(txnSaveStatuses, id)
	}
}

func awaitTxnSave(app *appstate.App, ids []string, done func(error)) {
	observeTxnDurability(app)
	app.RequestTransactionsDurable(ids, func(result appstate.DurableResult) { done(result.Err) })
}

// CommitUndoable is the shared transaction action boundary, also used by quick
// add at the app root. It captures preceding work, applies one validated atomic
// write, lets persistence capture the after-image, then advertises Undo after commit.
// A completion callback owns error presentation when supplied; otherwise this
// boundary emits exactly one error notice. Completion never crosses App scope.
func CommitUndoable(app *appstate.App, label string, ids []string, write func() error, done func(error)) error {
	return commitTxnAction(app, func(int) string { return label }, ids, write, false, done)
}

// CommitUndoableCounted names a durable action using the number of transaction
// records actually changed by its atomic write, excluding duplicates and no-ops.
func CommitUndoableCounted(app *appstate.App, label func(int) string, ids []string, write func() error, done func(error)) error {
	return commitTxnAction(app, label, ids, write, true, done)
}

func commitTxnAction(app *appstate.App, label func(int) string, ids []string, write func() error, requireRows bool, done func(error)) error {
	active := appstate.Default == app
	var changes []store.RecordChange
	count := 0
	finish := txnActionCompletion(app, func() string {
		if len(changes) == 0 || (requireRows && count == 0) {
			return ""
		}
		return label(count)
	}, true, "txint.changesNotSaved", done)
	if app == nil {
		err := appstate.ErrDurabilityUnavailable
		finish(err)
		return err
	}
	// Explicit App props also support isolated forms. Global undo/publication is
	// used only for the active App; the durable request always uses the given App.
	if active {
		auditview.CaptureNow()
	}
	before, err := app.Store().ChangeCursor()
	if err != nil {
		finish(err)
		return err
	}
	if err := write(); err != nil {
		app.Log().Error("transaction action failed", "err", err)
		finish(err)
		return err
	}
	changes, err = txnActionChanges(app, before)
	if err != nil {
		finish(err)
		return err
	}
	for _, change := range changes {
		if change.Table == "transactions" {
			count++
		}
	}
	awaitTxnSave(app, ids, finish)
	return nil
}

func commitUndoable(label string, ids []string, write func() error, done ...func(error)) error {
	var complete func(error)
	if len(done) > 0 {
		complete = done[0]
	}
	return CommitUndoable(appstate.Default, label, ids, write, complete)
}

// txnActionChanges reads a fixed SQL-committed range before persistence can prune
// it. Rolled-back writes never enter this journal; coalescing removes net no-ops.
func txnActionChanges(app *appstate.App, before uint64) ([]store.RecordChange, error) {
	target, err := app.Store().ChangeCursor()
	if err != nil {
		return nil, err
	}
	var changes []store.RecordChange
	for before < target {
		page, err := app.Store().CaptureChanges(before, target, 256)
		if err != nil {
			return nil, err
		}
		changes = append(changes, page.Changes...)
		before = page.Through
	}
	return store.CoalesceChanges(changes), nil
}

// CommitUndoableReplay is the asynchronous undo form of the commit boundary.
// replay must complete only after its restoration is durable (or has failed).
// The history owner retains failed restorations for retry and suppresses capture
// of replay as a new forward undo step. A replay never advertises another Undo.
func CommitUndoableReplay(app *appstate.App, label string, replay func(func(error)), done func(error)) {
	finish := txnActionCompletion(app, func() string { return label }, false, "tv8h.undoNotSaved", done)
	if app == nil {
		finish(appstate.ErrDurabilityUnavailable)
		return
	}
	replay(finish)
}

// txnActionCompletion is shared by atomic forward writes and asynchronous
// history replay. Only their durable completion may publish success, exactly
// once, and only within the App scope where the action began.
func txnActionCompletion(app *appstate.App, label func() string, undoable bool, failureKey string, done func(error)) func(error) {
	scope := appstate.Default
	completed := false
	return func(err error) {
		if completed || appstate.Default != scope {
			return
		}
		completed = true
		if app == scope {
			if err == nil && app != nil {
				uistate.BumpDataRevision()
				if text := label(); text != "" {
					uistate.PostDurable(text, undoable)
				}
			} else if done == nil {
				uistate.PostNotice(uistate.T(failureKey), true)
			}
		}
		if done != nil {
			done(err)
		}
	}
}
