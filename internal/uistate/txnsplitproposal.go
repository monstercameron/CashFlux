// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/CashFlux/internal/domain"

// txnSplitProposal is a receipt-derived split proposal (XC11) waiting to pre-fill
// the split editor. It is a plain package var, not an atom: it is written once (when
// vision extraction finishes) right before SetTxnSplit opens the modal, and read
// once by TransactionSplitForm when it seeds the editor. Keeping it off the reactive
// graph avoids re-seeding the editor on every host re-render.
var txnSplitProposal struct {
	txnID  string
	splits []domain.CategorySplit
	note   string
}

// SetTxnSplitProposal stashes a proposed category breakdown for txnID and opens the
// split-into-categories modal on that transaction. The editor pre-fills with these
// lines for the user to review and save (preview-then-approve) — nothing is
// persisted until the user saves.
func SetTxnSplitProposal(txnID string, splits []domain.CategorySplit, note string) {
	txnSplitProposal.txnID = txnID
	txnSplitProposal.splits = splits
	txnSplitProposal.note = note
	SetTxnSplit(txnID)
}

// TakeTxnSplitProposal returns and clears the pending proposal for txnID, if one is
// waiting. The bool is false when there is no proposal for that transaction (a plain
// manual split), so the caller seeds from the transaction's existing splits instead.
func TakeTxnSplitProposal(txnID string) ([]domain.CategorySplit, string, bool) {
	if txnSplitProposal.txnID == "" || txnSplitProposal.txnID != txnID {
		return nil, "", false
	}
	splits, note := txnSplitProposal.splits, txnSplitProposal.note
	txnSplitProposal.txnID = ""
	txnSplitProposal.splits = nil
	txnSplitProposal.note = ""
	return splits, note, true
}
