// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — transaction-page shared atoms.
//
// The /transactions page is composed entirely of widget-engine widgets (a
// toolbar tile, a ledger-table tile, a bulk-action tile, …) rendered through the
// spec/render pipeline, rather than one screen embedding a single widget. Because
// each block is now an independent engine widget, the interaction state they used
// to share through screen-local hooks (the bulk selection, the active sub-view,
// the pending undo, the receipt being previewed) lives here as shared atoms so any
// tile can read or mutate it and every other tile re-renders in step.
//
// The transaction FILTER already has its own shared atom (txfilter.go); these are
// the remaining pieces of cross-tile state.
package uistate

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	txnSelectionAtomID = "transactions:selection"
	txnSelAnchorAtomID = "transactions:selAnchor"
	txnBulkCatAtomID   = "transactions:bulkCat"
	txnViewAtomID      = "transactions:view"
	txnUndoAtomID      = "transactions:undo"
	txnPreviewAtomID   = "transactions:preview"
)

// TxnViewLedger / Import / Duplicates are the mutually exclusive sub-views the
// transactions surface can show in its main tile slot. Ledger is the default.
const (
	TxnViewLedger     = "ledger"
	TxnViewImport     = "import"
	TxnViewDuplicates = "duplicates"
)

// BulkSnapshot is the before-state of the last bulk operation, captured so the
// undo tile can restore it. Label is the human-readable description shown in the
// undo banner ("Recategorized 5 transactions"); Prior holds the affected
// transactions exactly as they were before the op.
type BulkSnapshot struct {
	Label string
	Prior []domain.Transaction
}

// UseTxnSelection returns the shared atom holding the set of selected transaction
// ids (id → true). The ledger-table tile toggles entries; the bulk-action tile
// reads them to act on the selection and the surface host reads them to decide
// whether to show the bulk tile at all.
func UseTxnSelection() state.Atom[map[string]bool] {
	return state.UseAtom(txnSelectionAtomID, map[string]bool{})
}

// UseTxnSelAnchor returns the shared atom holding the id of the last row toggled,
// the anchor a shift-click range select resolves against (in visible order).
func UseTxnSelAnchor() state.Atom[string] { return state.UseAtom(txnSelAnchorAtomID, "") }

// UseTxnBulkCat returns the shared atom holding the category id chosen in the
// bulk-action tile's "recategorize to" picker (empty = uncategorized).
func UseTxnBulkCat() state.Atom[string] { return state.UseAtom(txnBulkCatAtomID, "") }

// UseTxnView returns the shared atom selecting the active sub-view (ledger /
// import / duplicates). The toolbar tile toggles it; the host swaps which tile
// fills the main slot accordingly.
func UseTxnView() state.Atom[string] { return state.UseAtom(txnViewAtomID, TxnViewLedger) }

// UseTxnUndo returns the shared atom holding the last bulk operation's snapshot.
// A snapshot with no Prior rows means "nothing to undo" (the undo tile is hidden).
func UseTxnUndo() state.Atom[BulkSnapshot] { return state.UseAtom(txnUndoAtomID, BulkSnapshot{}) }

// UseTxnPreview returns the shared atom holding the receipt attachment currently
// open in the preview overlay. A zero ref (empty ArtifactID) means no preview.
func UseTxnPreview() state.Atom[domain.AttachmentRef] {
	return state.UseAtom(txnPreviewAtomID, domain.AttachmentRef{})
}
