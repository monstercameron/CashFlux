// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"time"

	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	txnViewModeAtomID = "transactions:viewMode"
	txnRegisterAtomID = "transactions:register"
	txnCalMonthAtomID = "transactions:calMonth"
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

// UseTxnCalMonth returns the shared atom holding the month the calendar view is
// paged to (any day within the month). A zero value means "the current month" —
// the calendar body resolves that to time.Now at render so it always opens on the
// present month.
func UseTxnCalMonth() state.Atom[time.Time] { return state.UseAtom(txnCalMonthAtomID, time.Time{}) }
