// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"sync/atomic"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// activeTxnFocusCallbacks is exposed to package tests as a seam for detecting
// deferred-focus callbacks that outlive their one-shot timer.
var activeTxnFocusCallbacks atomic.Int32

type txnKeyAction uint8

const (
	txnKeyNone txnKeyAction = iota
	txnKeySearch
	txnKeyNext
	txnKeyPrevious
	txnKeySelect
	txnKeyEdit
	txnKeyCategory
	txnKeyReview
	txnKeyHelp
)

const txnCategoryEditSelector = "#txn-edit-form [data-testid='txn-edit-category'], #txn-edit-form .txn-cat-row select"

func txnActionForKey(key string) txnKeyAction {
	switch key {
	case "/":
		return txnKeySearch
	case "j":
		return txnKeyNext
	case "k":
		return txnKeyPrevious
	case "x":
		return txnKeySelect
	case "e", "Enter":
		return txnKeyEdit
	case "c":
		return txnKeyCategory
	case "r":
		return txnKeyReview
	case "?":
		return txnKeyHelp
	default:
		return txnKeyNone
	}
}

func txnShortcutBlocked(editable, dialogOpen bool) bool {
	return editable || dialogOpen
}

func txnShortcutConsumedWithoutAction(editable, dialogOpen bool, key string) bool {
	return !editable && dialogOpen && txnActionForKey(key) != txnKeyNone
}

func txnHelpKeyCloses(key string, guideOpen bool) bool {
	return key == "?" && guideOpen
}

func txnHelpShouldClose(doc js.Value, key string) bool {
	dialog := doc.Call("getElementById", "txn-key-help")
	guideOpen := !dialog.IsNull() && !dialog.IsUndefined()
	return txnHelpKeyCloses(key, guideOpen)
}

// HandleTxnKey runs a ledger shortcut when the transaction table is mounted.
// The app's global dispatcher already filters modifiers and editable targets;
// this boundary repeats the safety checks so it remains safe if called directly.
func HandleTxnKey(key string) bool {
	if txnActionForKey(key) == txnKeyNone {
		return false
	}
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return false
	}
	editable, dialogOpen := txnEditableTarget(doc), txnDialogOpen(doc)
	if txnShortcutBlocked(editable, dialogOpen) {
		if txnHelpShouldClose(doc, key) {
			SetTxnKeyHelpOpen(false)
			return true
		}
		// Consume shortcuts over a dialog so the app-level '?' handler cannot open
		// a second overlay. Editable controls keep their typed characters.
		return txnShortcutConsumedWithoutAction(editable, dialogOpen, key)
	}
	table := doc.Call("querySelector", ".txn-table")
	if table.IsNull() || table.IsUndefined() {
		return false
	}

	rows := table.Call("querySelectorAll", "tbody tr.row")
	active := doc.Get("activeElement")
	row := closestTxnRow(active)
	idx := -1
	for i := 0; i < rows.Length(); i++ {
		if rows.Index(i).Equal(row) {
			idx = i
			break
		}
	}
	hasFocusedRow := idx >= 0
	if idx < 0 && rows.Length() > 0 {
		idx = 0
		row = rows.Index(0)
	}

	switch txnActionForKey(key) {
	case txnKeySearch:
		search := doc.Call("querySelector", "input[data-testid='filter-toolbar-search']")
		if search.IsNull() || search.IsUndefined() {
			return false
		}
		search.Call("focus")
		return true
	case txnKeyNext, txnKeyPrevious:
		if rows.Length() == 0 {
			return false
		}
		idx = txnNextRowIndex(idx, rows.Length(), key, hasFocusedRow)
		row = rows.Index(idx)
		row.Call("setAttribute", "tabindex", "-1")
		row.Call("focus")
		return true
	case txnKeySelect:
		if row.IsNull() || row.IsUndefined() {
			return false
		}
		checkbox := row.Call("querySelector", "input[type='checkbox']")
		if checkbox.IsNull() || checkbox.IsUndefined() {
			return false
		}
		checkbox.Call("click")
		return true
	case txnKeyEdit, txnKeyCategory:
		if row.IsNull() || row.IsUndefined() {
			return false
		}
		testID := row.Call("getAttribute", "data-testid")
		if testID.IsNull() || testID.IsUndefined() {
			return false
		}
		id, ok := txnRowID(testID.String())
		if !ok {
			return false
		}
		uistate.SetTxnEdit(id)
		if txnActionForKey(key) == txnKeyCategory {
			scheduleTxnFocus(doc, txnCategoryEditSelector)
		}
		return true
	case txnKeyReview:
		if row.IsNull() || row.IsUndefined() {
			return false
		}
		testID := row.Call("getAttribute", "data-testid")
		if testID.IsNull() || testID.IsUndefined() {
			return false
		}
		id, ok := txnRowID(testID.String())
		if !ok {
			return false
		}
		markTxnReviewedFromKeyboard(id)
		return true
	case txnKeyHelp:
		SetTxnKeyHelpOpen(true)
		return true
	}
	return false
}

func txnNextRowIndex(idx, rowCount int, key string, hasFocusedRow bool) int {
	if rowCount == 0 {
		return -1
	}
	if !hasFocusedRow {
		if key == "j" {
			return 0
		}
		return rowCount - 1
	}
	if key == "j" {
		idx++
	} else {
		idx--
	}
	if idx < 0 {
		return rowCount - 1
	}
	if idx >= rowCount {
		return 0
	}
	return idx
}

func txnRowID(testID string) (string, bool) {
	if !strings.HasPrefix(testID, "txn-row-") {
		return "", false
	}
	id := strings.TrimPrefix(testID, "txn-row-")
	return id, id != ""
}

func txnEditableTarget(doc js.Value) bool {
	active := doc.Get("activeElement")
	if active.IsNull() || active.IsUndefined() {
		return false
	}
	tag := active.Get("tagName")
	if !tag.IsNull() && !tag.IsUndefined() {
		switch strings.ToUpper(tag.String()) {
		case "INPUT", "TEXTAREA", "SELECT":
			return true
		}
	}
	editable := active.Get("isContentEditable")
	return !editable.IsNull() && !editable.IsUndefined() && editable.Bool()
}

func txnDialogOpen(doc js.Value) bool {
	dialogs := doc.Call("querySelectorAll", "[role='dialog']")
	for i := 0; i < dialogs.Length(); i++ {
		d := dialogs.Index(i)
		if d.Get("hidden").Bool() {
			continue
		}
		style := js.Global().Get("getComputedStyle").Invoke(d)
		if style.Get("display").String() != "none" && style.Get("visibility").String() != "hidden" {
			return true
		}
	}
	return false
}

func closestTxnRow(el js.Value) js.Value {
	if el.IsNull() || el.IsUndefined() {
		return js.Null()
	}
	return el.Call("closest", ".txn-table tbody tr.row")
}

func scheduleTxnFocus(doc js.Value, selector string) {
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		defer cb.Release()
		defer activeTxnFocusCallbacks.Add(-1)
		target := doc.Call("querySelector", selector)
		if !target.IsNull() && !target.IsUndefined() {
			target.Call("focus")
		}
		return nil
	})
	activeTxnFocusCallbacks.Add(1)
	js.Global().Call("setTimeout", cb, 0)
}
