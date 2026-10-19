// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/auditview"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
	scope := appstate.Default
	active := scope == app
	finish := func(err error) {
		if appstate.Default != scope {
			return
		}
		if done != nil {
			done(err)
		} else if active && err != nil {
			uistate.PostNotice(uistate.T("txint.changesNotSaved"), true)
		}
	}
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
	if err := write(); err != nil {
		if done == nil && active {
			uistate.PostNotice(err.Error(), true)
		} else {
			finish(err)
		}
		return err
	}
	awaitTxnSave(app, ids, func(err error) {
		if appstate.Default != scope {
			return
		}
		if err == nil && active {
			uistate.BumpDataRevision()
			uistate.PostUndoable(uistate.T("toast.undoStory", label))
		}
		finish(err)
	})
	return nil
}

func commitUndoable(label string, ids []string, write func() error, done ...func(error)) error {
	var complete func(error)
	if len(done) > 0 {
		complete = done[0]
	}
	return CommitUndoable(appstate.Default, label, ids, write, complete)
}
