// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"strings"
	"sync"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeebase"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/taskresolve"
)

// lastAutoResolve is the single-step session-undo snapshot for the most recent
// auto-resolve pass (the dataset just before it completed tasks) plus the titles
// it closed, so the "Done for you" toast can be undone within the session.
var lastAutoResolve *lastAutoResolveState

type lastAutoResolveState struct {
	app      *App
	changes  []store.RecordChange
	snapshot *store.Dataset
	titles   []string
}

// taskResolveVarsCache reuses the dataset-wide workflow variables across the
// consecutive transaction events emitted by one import batch. Every store
// mutation advances Rev, so the cached view is discarded before a later write
// can observe stale dataset state. App operations run on the wasm thread; the
// mutex also keeps native callers safe when independent apps resolve at once.
var taskResolveVarsCache struct {
	sync.Mutex
	app  *App
	rev  uint64
	vars map[string]float64
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
	tasks := a.Tasks()
	if !hasOpenResolveTask(tasks) {
		return
	}
	vars := a.taskResolveVars()
	defs := a.CustomFieldDefsFor("transaction")
	ev := taskresolve.Event{
		Vars: taskResolveEventVars(vars, t, a.Transactions(), defs),
		Strs: map[string]string{},
		Txn: &taskresolve.TxnEvent{
			Payee:       txnPayeeOrDesc(t),
			AmountMinor: t.Amount.Amount,
			Currency:    t.Amount.Currency,
		},
	}
	// Enrich Strs with the txn's payee/desc/category/account, mirroring txnContext
	// so a condition can reference the same names.
	addTaskResolveTransactionStrings(a, ev.Strs, t, defs)

	var resolved []domain.Task
	for _, tk := range tasks {
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
		st := &lastAutoResolveState{app: a, snapshot: &snap}
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

// hasOpenResolveTask reports whether any task can be considered by the hook.
func hasOpenResolveTask(tasks []domain.Task) bool {
	for _, task := range tasks {
		if task.Status == domain.StatusOpen && task.Resolve != nil {
			return true
		}
	}
	return false
}

// taskResolveVars returns the engine variables for the current store revision.
func (a *App) taskResolveVars() map[string]float64 {
	rev := a.store.Rev()
	taskResolveVarsCache.Lock()
	defer taskResolveVarsCache.Unlock()
	if taskResolveVarsCache.app == a && taskResolveVarsCache.rev == rev {
		return taskResolveVarsCache.vars
	}
	vars := a.engineVars()
	taskResolveVarsCache.app = a
	taskResolveVarsCache.rev = rev
	taskResolveVarsCache.vars = vars
	return vars
}

// taskResolveEventVars combines the cached dataset variables with the
// transaction-specific workflow fields for one posting event.
func taskResolveEventVars(base map[string]float64, txn domain.Transaction, transactions []domain.Transaction, defs []customfields.Def) map[string]float64 {
	vars := make(map[string]float64, len(base)+len(defs)+4)
	for name, value := range base {
		vars[name] = value
	}
	div := 1.0
	for i := 0; i < currency.Decimals(txn.Amount.Currency); i++ {
		div *= 10
	}
	amount := float64(txn.Amount.Amount) / div
	vars["txn_amount"] = amount
	vars["txn_abs"] = math.Abs(amount)
	if typical, ok := payeebase.Typical(transactions, txn); ok {
		vars["txn_payee_typical"] = float64(typical) / div
		vars["txn_vs_typical"] = math.Abs(float64(txn.Amount.Amount)) / float64(typical)
	} else {
		vars["txn_payee_typical"] = 0
		vars["txn_vs_typical"] = 0
	}
	for _, def := range defs {
		name := "cf_txn_" + def.Key
		switch def.Type {
		case customfields.TypeNumber:
			vars[name] = 0
			switch n := txn.Custom[def.Key].(type) {
			case float64:
				vars[name] = n
			case int:
				vars[name] = float64(n)
			}
		case customfields.TypeBool:
			vars[name] = 0
			if value, ok := txn.Custom[def.Key].(bool); ok && value {
				vars[name] = 1
			}
		}
	}
	return vars
}

// addTaskResolveTransactionStrings adds the transaction-name fields used by
// workflow conditions to the event's string-variable map.
func addTaskResolveTransactionStrings(a *App, strs map[string]string, txn domain.Transaction, defs []customfields.Def) {
	strs["txn_payee"] = txn.Payee
	strs["txn_desc"] = txn.Desc
	strs["txn_tags"] = strings.Join(txn.Tags, ",")
	for _, account := range a.Accounts() {
		if account.ID == txn.AccountID {
			strs["txn_account"] = account.Name
			break
		}
	}
	for _, category := range a.Categories() {
		if category.ID == txn.CategoryID {
			strs["txn_category"] = category.Name
			break
		}
	}
	for _, def := range defs {
		if def.Type == customfields.TypeNumber || def.Type == customfields.TypeBool {
			continue
		}
		value, _ := txn.Custom[def.Key].(string)
		strs["cf_txn_"+def.Key] = value
	}
}

// UndoLastAutoResolve reverts the most recent auto-resolve pass, restoring the
// tasks it closed. It returns the titles that were restored (for a confirmation)
// and whether an undo was available. Single-step: only the latest pass can be
// undone, and it is consumed once used.
func (a *App) UndoLastAutoResolve() ([]string, bool) {
	if lastAutoResolve == nil || lastAutoResolve.app != a {
		return nil, false
	}
	st := lastAutoResolve
	lastAutoResolve = nil
	if len(st.changes) > 0 {
		undo := make([]store.RecordChange, 0, len(st.changes))
		for i := len(st.changes) - 1; i >= 0; i-- {
			change := st.changes[i]
			change.Before, change.After = change.After, change.Before
			undo = append(undo, change)
		}
		if err := a.store.ReplayChangesExpected(undo); err != nil {
			a.logErr("autoResolveUndoChanges", err)
			return nil, false
		}
		a.fireTxnMutated()
		return st.titles, true
	}
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
