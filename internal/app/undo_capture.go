// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

var undoCaptureCursor uint64
var undoPending []pendingUndoCapture

type pendingUndoCapture struct {
	through uint64
	change  history.ChangeSet
}

var recordCollections = map[string]string{
	"transfer_operations": "transferOperations",
	"members":             "members", "accounts": "accounts", "categories": "categories", "transactions": "transactions",
	"budgets": "budgets", "goals": "goals", "holdings": "holdings", "realizedsales": "realizedSales",
	"balance_snapshots": "balanceSnapshots", "tasks": "tasks", "customfielddefs": "customFieldDefs", "rules": "rules",
	"documents": "documents", "savedinsights": "savedInsights", "conversations": "conversations", "recurring": "recurring",
	"allocprofiles": "allocProfiles", "formulas": "formulas", "plans": "plans", "custompages": "customPages",
	"placements": "placements", "molecules": "molecules", "artifacts": "artifacts", "workflows": "workflows",
	"workflowruns": "workflowRuns", "sharedexpenses": "sharedExpenses", "settlements": "settlements", "earmarks": "earmarks",
	"txnlinks": "txnLinks", "events": "events", "accountgroups": "accountGroups", "payeealiases": "payeeAliases",
	"subcancellations": "subscriptionCancellations", "subignores": "subscriptionIgnores", "institutions": "institutions",
	"sweeprules": "sweepRules", "debt_payment_links": "debtPaymentLinks", "debt_payee_rules": "debtPayeeRules", "audit_log": "auditEntries",
}

// applyCapturedRecords touches only changed snapshot rows; the returned diff
// retains full scalar-KV values for compatibility with existing history undo.
func applyCapturedRecords(snapshot history.Snapshot, records []store.RecordChange) (history.ChangeSet, error) {
	before, after := history.Snapshot{}, history.Snapshot{}
	for _, record := range records {
		collection, id := recordCollections[record.Table], record.ID
		value := record.After
		switch record.Table {
		case "transfer_operations":
			if record.After != nil {
				var transactions []domain.Transaction
				if err := json.Unmarshal(record.After, &transactions); err != nil {
					return history.ChangeSet{}, err
				}
				var err error
				value, err = json.Marshal(store.TransferOperationReceipt{ID: record.ID, Transactions: transactions})
				if err != nil {
					return history.ChangeSet{}, err
				}
			}
		case "settings":
			collection, id = "_meta:settings", "settings"
		case "appkv", "settingskv":
			id = "appState"
			if record.Table == "settingskv" {
				id = "settingsState"
			}
			collection = "_meta:" + id
			values := map[string]string{}
			if raw := snapshot[collection][id]; len(raw) > 0 {
				if err := json.Unmarshal(raw, &values); err != nil {
					return history.ChangeSet{}, fmt.Errorf("decode undo KV: %w", err)
				}
			}
			if values == nil {
				values = map[string]string{}
			}
			if record.After == nil {
				delete(values, record.ID)
			} else {
				var text string
				if err := json.Unmarshal(record.After, &text); err != nil {
					return history.ChangeSet{}, err
				}
				values[record.ID] = text
			}
			var err error
			value, err = json.Marshal(values)
			if err != nil {
				return history.ChangeSet{}, err
			}
		}
		if collection == "" {
			continue
		} // SQL receipt/membership tables are not dataset collections.
		if record.Table != "settings" && record.Table != "appkv" && record.Table != "settingskv" {
			delete(snapshot, "_meta:"+collection)
		}
		if before[collection] == nil {
			before[collection] = map[string]json.RawMessage{}
			after[collection] = map[string]json.RawMessage{}
		}
		if _, touched := after[collection][id]; !touched {
			if old := snapshot[collection][id]; old != nil {
				before[collection][id] = old
			}
		}
		if snapshot[collection] == nil {
			snapshot[collection] = map[string]json.RawMessage{}
		}
		if value == nil {
			delete(snapshot[collection], id)
			delete(after[collection], id)
		} else {
			snapshot[collection][id] = value
			after[collection][id] = value
		}
	}
	return history.Diff(before, after), nil
}

func captureIncrementalUndo(app *appstate.App) error {
	_, err := captureIncrementalUndoReceipt(app)
	return err
}

// captureIncrementalUndoReceipt uses the same canonical filtered capture as the
// autosave path and returns the audit/undo ID only when it stages a real point.
func captureIncrementalUndoReceipt(app *appstate.App) (string, error) {
	target, err := app.Store().ChangeCursor()
	if err != nil {
		return "", err
	}
	var records []store.RecordChange
	for from := undoCaptureCursor; from < target; {
		page, err := app.Store().CaptureChanges(from, target, 128)
		if err != nil {
			return "", err
		}
		records = append(records, page.Changes...)
		from = page.Through
	}
	cs, err := applyCapturedRecords(lastUndoSnap, records)
	if err != nil {
		return "", err
	}
	undoCaptureCursor = target
	cs = filterCapturedChanges(cs)
	undoID := ""
	if !cs.IsEmpty() {
		undoID = RecordAuditPoint(cs)
		cs.UndoID = undoID
		undoPending = append(undoPending, pendingUndoCapture{through: target, change: cs})
	}
	lastUndoCaptureApp, lastUndoCaptureRev = app, app.Rev()
	return undoID, nil
}

// captureAssistantBaseline forces the ordinary household capture before an
// approved Assistant tool starts. This keeps any earlier pending manual writes
// separate and preserves the actor attached when each pending point was made.
func captureAssistantBaseline() bool {
	if applyingUndo {
		return false
	}
	app := appstate.Default
	if app == nil {
		return false
	}
	if lastUndoCaptureApp != app || lastUndoSnap == nil {
		initUndo()
		return lastUndoCaptureApp == app && lastUndoSnap != nil
	}
	if _, err := captureIncrementalUndoReceipt(app); err != nil {
		app.Log().Error("capture assistant baseline failed", "err", err)
		return false
	}
	return true
}

// captureAssistantReceipt forces one capture after the synchronous Assistant
// tool run. Store changes since the baseline are therefore exactly this tool's
// mutation, and only a nonempty filtered diff receives a receipt.
func captureAssistantReceipt() (string, bool) {
	if applyingUndo {
		return "", false
	}
	app := appstate.Default
	if app == nil {
		return "", false
	}
	if lastUndoCaptureApp != app || lastUndoSnap == nil {
		initUndo()
		return "", false
	}
	undoID, err := captureIncrementalUndoReceipt(app)
	if err != nil {
		app.Log().Error("capture assistant change failed", "err", err)
		return "", false
	}
	return undoID, undoID != ""
}

func publishDurableUndo(through uint64) {
	if undoPersistencePending && through >= undoDurableCursor {
		undoPersistencePending = false
	}
	i := 0
	for i < len(undoPending) && undoPending[i].through <= through {
		undoStack.Push(undoPending[i].change)
		i++
	}
	undoPending = undoPending[i:]
	if i > 0 {
		uistate.NotifyAssistantUndoStateChanged()
	}
}
