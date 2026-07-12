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
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v4/state"
)

const (
	txnSelectionAtomID = "transactions:selection"
	txnSelAnchorAtomID = "transactions:selAnchor"
	txnBulkCatAtomID   = "transactions:bulkCat"
	txnBulkMemAtomID   = "transactions:bulkMember"
	txnUndoAtomID      = "transactions:undo"
	txnPreviewAtomID   = "transactions:preview"
	txnColsAtomID      = "transactions:cols"
	txnColsModalAtomID = "transactions:colsModal"
	txnColsStoreID     = "cashflux:txn-cols"
	txnSmartCatAtomID  = "transactions:smartCat"
	txnLinkAtomID      = "transactions:linkTarget"
)

// Payment-link modal modes: the flip modal a transaction row's ⋯ menu opens can link
// the transaction to a liability (bill payment) or a subscription.
const (
	TxnLinkModeBill = "bill"
	TxnLinkModeSub  = "sub"
)

// TxnLinkTarget identifies the transaction whose payment-link flip modal is open and
// which mode it opened to (Bill or Subscription — the modal offers a toggle between
// them). A zero TxnID means the modal is closed.
type TxnLinkTarget struct {
	TxnID string
	Mode  string
}

// UseTxnLinkTarget returns the shared atom driving the payment-link flip modal. A row
// ⋯ menu item sets it (with the row's id + the chosen mode); the shell-root host
// renders the modal when TxnID is non-empty.
func UseTxnLinkTarget() state.Atom[TxnLinkTarget] {
	return state.UseAtom(txnLinkAtomID, TxnLinkTarget{})
}

// TxnCols selects which optional ledger columns are visible. Date and Description
// are the row's identity and always shown, so they are not toggleable here.
type TxnCols struct {
	Amount   bool `json:"amount"`
	Account  bool `json:"account"`
	Category bool `json:"category"`
	Source   bool `json:"source"`
	User     bool `json:"user"`
}

// DefaultTxnCols shows every optional column (the ledger's historical layout, plus
// the new User column).
func DefaultTxnCols() TxnCols {
	return TxnCols{Amount: true, Account: true, Category: true, Source: true, User: true}
}

// UseTxnCols returns the shared atom holding the ledger's column visibility,
// seeded from localStorage. The table tile reads it; the columns modal writes it.
func UseTxnCols() state.Atom[TxnCols] {
	return state.UseAtom(txnColsAtomID, loadTxnCols())
}

// PersistTxnCols remembers the column visibility across reloads.
func PersistTxnCols(c TxnCols) {
	if b, err := json.Marshal(c); err == nil {
		SettingKVSet(txnColsStoreID, string(b))
	}
}

func loadTxnCols() TxnCols {
	raw := SettingKVGet(txnColsStoreID)
	if raw == "" {
		return DefaultTxnCols()
	}
	c := DefaultTxnCols()
	_ = json.Unmarshal([]byte(raw), &c)
	return c
}

// UseTxnColsModalOpen returns the shared atom selecting whether the "show/hide
// columns" flip modal is open. The toolbar's Columns button sets it; the host
// tile renders the flip modal when true.
func UseTxnColsModalOpen() state.Atom[bool] { return state.UseAtom(txnColsModalAtomID, false) }

// UseTxnSmartCatOpen returns the shared atom selecting whether the Smart+
// categorization review modal is open. The toolbar's "Smart categorize" button
// sets it; the shell-root host renders the flip modal when true.
func UseTxnSmartCatOpen() state.Atom[bool] { return state.UseAtom(txnSmartCatAtomID, false) }

// UseImportPanelOpen returns the shared atom selecting whether the single, merged
// "Import" flip modal is open. The transactions toolbar's Import button sets it; the
// shell-root ImportPanelHost renders the modal when true. This is now the ONLY import
// entry point — a two-stage wizard (add-your-data hub → review) that absorbed both the
// former in-place CSV/receipt panel and the standalone statement-PDF ("Import
// statement") modal.
func UseImportPanelOpen() state.Atom[bool] {
	return state.UseAtom("transactions:importPanel", false)
}

// UseDuplicatesModalOpen returns the shared atom selecting whether the "Review
// duplicates" flip modal is open. The transactions toolbar's duplicates button sets it;
// the shell-root DuplicatesHost renders the modal when true. (Duplicates used to be an
// in-place TxnViewDuplicates sub-view that took over the ledger slot; like import, it's
// now a shell-root flip modal over the ledger.)
func UseDuplicatesModalOpen() state.Atom[bool] {
	return state.UseAtom("transactions:duplicatesModal", false)
}

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

// UseTxnBulkMember returns the shared atom holding the member id chosen in the
// bulk-action tile's "assign to" picker (empty = nobody / unassigned).
func UseTxnBulkMember() state.Atom[string] { return state.UseAtom(txnBulkMemAtomID, "") }

// UseTxnUndo returns the shared atom holding the last bulk operation's snapshot.
// A snapshot with no Prior rows means "nothing to undo" (the undo tile is hidden).
func UseTxnUndo() state.Atom[BulkSnapshot] { return state.UseAtom(txnUndoAtomID, BulkSnapshot{}) }

// UseTxnPreview returns the shared atom holding the receipt attachment currently
// open in the preview overlay. A zero ref (empty ArtifactID) means no preview.
func UseTxnPreview() state.Atom[domain.AttachmentRef] {
	return state.UseAtom(txnPreviewAtomID, domain.AttachmentRef{})
}
