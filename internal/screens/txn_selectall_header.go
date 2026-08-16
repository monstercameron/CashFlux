// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// hiddenSortKeys maps the user's column-visibility choices onto the sort keys
// those columns carry, so txnfilter.EffectiveSort can tell which sorts currently
// have no header to show them.
//
// Only the four hideable sortable columns appear. Date and Description have no
// visibility toggle, so their keys are never in the map and are always treated as
// visible — which is what an absent key means.
func hiddenSortKeys(cols uistate.TxnCols) map[string]bool {
	return map[string]bool{
		"amount":   !cols.Amount,
		"account":  !cols.Account,
		"category": !cols.Category,
		"source":   !cols.Source,
	}
}

// txnSelectAllHeaderProps carries the ids of the rows currently on screen. It is
// the VISIBLE set, not the filtered set: the page you are looking at, or the whole
// list while the "All" view is virtualized.
type txnSelectAllHeaderProps struct {
	Visible []string
}

// txnSelectAllHeader is the ledger's header checkbox: one control that selects or
// clears every row currently in view.
//
// Selecting a page used to mean clicking each row, or reaching into "⋯ More →
// Select all" — which selects the whole FILTERED set across every page, a
// different and much larger act than the one a user usually wants. A checkbox in
// the header column, directly above the per-row checkboxes it controls, is where
// people look for this.
//
// It is a three-state control, because two states would lie: with some of the page
// selected, an unchecked box says "nothing here is selected". The middle state is
// the DOM's `indeterminate` property, which has no HTML attribute — it can only be
// set from script — so the component owns a stable id and writes the property in an
// effect after each render.
func txnSelectAllHeader(props txnSelectAllHeaderProps) ui.Node {
	selAtom := uistate.UseTxnSelection()
	anchorAtom := uistate.UseTxnSelAnchor()
	id := ui.UseId()

	sel := selAtom.Get()
	selected := 0
	for _, rowID := range props.Visible {
		if sel[rowID] {
			selected++
		}
	}
	total := len(props.Visible)
	all := total > 0 && selected == total
	some := selected > 0 && !all

	// Clicking commits the state the box is NOT in: a full or partial selection
	// clears the visible rows, anything else selects them. Rows outside the view
	// keep whatever they had — this control speaks only for what is on screen, and
	// silently dropping an off-page selection would be a bigger act than the one
	// the user asked for.
	toggle := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		cur := selAtom.Get()
		next := make(map[string]bool, len(cur)+total)
		for k, v := range cur {
			if v {
				next[k] = v
			}
		}
		for _, rowID := range props.Visible {
			if all {
				delete(next, rowID)
			} else {
				next[rowID] = true
			}
		}
		selAtom.Set(next)
		// The shift-click anchor belongs to a row the user picked; a bulk toggle is
		// not that, so it is cleared rather than left pointing at a stale row.
		anchorAtom.Set("")
	})

	// `indeterminate` is a property, never an attribute, so it survives no amount of
	// markup — it has to be written after the checkbox exists. Keyed on the state so
	// the write happens whenever the tri-state changes and not on every render.
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil
		}
		el := doc.Call("getElementById", id)
		if el.Truthy() {
			el.Set("indeterminate", some)
		}
		return nil
	}, boolStr(some)+"|"+boolStr(all)+"|"+strconv.Itoa(total))

	label := uistate.T("transactions.selectAllVisible", total)
	if all {
		label = uistate.T("transactions.clearAllVisible", total)
	} else if some {
		label = uistate.T("transactions.selectAllVisiblePartial", selected, total)
	}

	return Input(
		Type("checkbox"),
		Attr("id", id),
		Attr("data-testid", "txn-select-all-visible"),
		Attr("aria-label", label),
		Attr("title", label),
		css.Class("txn-selectall-box"),
		CheckedIf(all),
		OnClick(toggle),
	)
}
