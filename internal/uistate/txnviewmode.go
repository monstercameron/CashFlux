// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v5/state"

const (
	txnViewModeAtomID = "transactions:viewMode"
	txnRegisterAtomID = "transactions:register"
)

// Transaction view modes: the ledger surface renders either the sortable table
// (default) or a month calendar (TX8). The register running-balance column (TX12)
// is an orthogonal toggle that decorates the TABLE, so it has its own atom.
const (
	TxnViewTable    = "table"
	TxnViewCalendar = "calendar"
)

// UseTxnViewMode returns the shared atom selecting the ledger's view mode
// (TxnViewTable or TxnViewCalendar). The toolbar toggles it; the surface host
// swaps the main-slot widget on it. Not persisted — a view mode is a transient
// lens, and a reload sensibly lands back on the table.
func UseTxnViewMode() state.Atom[string] { return state.UseAtom(txnViewModeAtomID, TxnViewTable) }

// UseTxnRegisterMode returns the shared atom selecting whether register mode (the
// running-balance column, TX12) is on. It only takes effect when the filter is
// scoped to exactly one account; the toolbar hides the toggle otherwise.
func UseTxnRegisterMode() state.Atom[bool] { return state.UseAtom(txnRegisterAtomID, false) }

// Removed (C560): the calendar used to page a private month atom anchored on
// time.Now(), independent of the ledger's date filter — which is how the grid
// could show one month while the rows and the period label showed another, and
// how paging the calendar changed neither. The visible month is now DERIVED from
// the ledger's own From/To through internal/txnscope, so there is one period state
// on the surface instead of three. Nothing should reintroduce a second one.
