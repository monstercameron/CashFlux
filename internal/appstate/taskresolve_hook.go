// SPDX-License-Identifier: MIT

package appstate

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/taskresolve"
)

// lastAutoResolve is the single-step session-undo snapshot for the most recent
// auto-resolve pass (the dataset just before it completed tasks) plus the titles
// it closed, so the "Done for you" toast can be undone within the session.
var lastAutoResolve *lastAutoResolveState

type lastAutoResolveState struct {
	snapshot *store.Dataset
	titles   []string
}

// resolveTasksForTxn evaluates every open self-resolving task (domain.Task.Resolve)
// against the transaction that just posted and auto-completes the ones whose rule
// is satisfied (XC8). It runs on the store-mutation path (a data event), never on
// a timer. Each auto-completion posts a quiet, undoable "Done for you: …" notice
// via the Notifier seam. A malformed condition is logged and skipped — a broken
// rule never closes a task.
func (a *App) resolveTasksForTxn(t domain.Transaction) {
	if a.triggersSuspended {
		return // don't self-resolve while applying workflow effects
	}
	ev := taskresolve.Event{
		Vars: a.engineVars(),
		Strs: map[string]string{},
		Txn: &taskresolve.TxnEvent{
			Payee:       txnPayeeOrDesc(t),
			AmountMinor: t.Amount.Amount,
			Currency:    t.Amount.Currency,
		},
	}
	// Enrich Strs with the txn's payee/desc/category/account, mirroring txnContext
	// so a condition can reference the same names.
	ctx := a.txnContext(t)
	for k, v := range ctx.Strs {
		ev.Strs[k] = v
	}
	for k, v := range ctx.Vars {
		ev.Vars[k] = v
	}

	var resolved []domain.Task
	for _, tk := range a.Tasks() {
		if tk.Status != domain.StatusOpen || tk.Resolve == nil {
			continue
		}
		ok, err := taskresolve.Resolves(*tk.Resolve, ev)
		if err != nil {
			a.log.Warn("task resolve condition failed", "task", tk.ID, "err", err)
			continue
		}
		if ok {
			resolved = append(resolved, tk)
		}
	}
	if len(resolved) == 0 {
		return
	}
	// Snapshot for single-step undo before mutating.
	if snap, err := a.store.Snapshot(); err == nil {
		st := &lastAutoResolveState{snapshot: &snap}
		for _, tk := range resolved {
			st.titles = append(st.titles, tk.Title)
		}
		lastAutoResolve = st
	}
	for _, tk := range resolved {
		if err := a.CompleteTask(tk.ID, "", a.clock()); err != nil {
			a.logErr("autoResolveComplete", err)
			continue
		}
		a.log.Info("task auto-resolved", "task", tk.ID, "title", tk.Title)
		if a.Notifier != nil {
			a.Notifier("Done for you: " + tk.Title)
		}
	}
	a.fireTxnMutated()
}

// UndoLastAutoResolve reverts the most recent auto-resolve pass, restoring the
// tasks it closed. It returns the titles that were restored (for a confirmation)
// and whether an undo was available. Single-step: only the latest pass can be
// undone, and it is consumed once used.
func (a *App) UndoLastAutoResolve() ([]string, bool) {
	if lastAutoResolve == nil {
		return nil, false
	}
	st := lastAutoResolve
	lastAutoResolve = nil
	if st.snapshot == nil {
		return nil, false
	}
	if err := a.store.Load(*st.snapshot); err != nil {
		a.logErr("autoResolveUndoLoad", err)
		return nil, false
	}
	a.fireTxnMutated()
	return st.titles, true
}

// txnPayeeOrDesc returns the transaction's payee, falling back to its description.
func txnPayeeOrDesc(t domain.Transaction) string {
	if t.Payee != "" {
		return t.Payee
	}
	return t.Desc
}
