// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

// Import-in-progress markers (#62 "Continue where I left off").
//
// Two layers with different truths:
//   - UseImportDraftRows holds the LIVE review-stage rows in a session atom, so
//     navigating away from /documents mid-review no longer discards them and the
//     dashboard resume card can offer a real jump back into the wizard.
//   - The persisted WIP marker (a row count in KV) survives a reload. The rows
//     themselves don't, so after a reload the resume card phrases it honestly
//     ("the rows couldn't be kept — start it again") instead of promising a
//     resume it can't deliver.
//
// Both are written from the wizard's single draft choke point (setDraft in
// documents.go): rows > 0 stamps them, an emptied draft (imported, cleared, or
// last row removed) clears them.

// UseImportDraftRows returns the session atom holding the import wizard's
// review-stage rows.
func UseImportDraftRows() state.Atom[[]extract.Row] {
	return state.UseAtom("documents:draft-rows", []extract.Row(nil))
}

const importWIPStoreID = "cashflux:import-wip"

// SetImportWIP records that an import review with n rows is in progress.
func SetImportWIP(n int) { kvSet(importWIPStoreID, strconv.Itoa(n)) }

// ClearImportWIP removes the marker (import finished, cleared, or abandoned).
func ClearImportWIP() { kvSet(importWIPStoreID, "") }

// ImportWIPCount returns the persisted in-progress row count, 0 when none.
func ImportWIPCount() int {
	n, err := strconv.Atoi(kvGet(importWIPStoreID))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
