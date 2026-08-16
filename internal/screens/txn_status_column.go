// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "github.com/monstercameron/CashFlux/internal/uistate"

// rowStatusWord is the ledger Status column's cell text: the row's reconciled /
// cleared / needs-review state spelled out (C578).
//
// It says the same thing as the inline glyph, in words, in a lane where a repeated
// value is expected. The two must not disagree, so the branch order here mirrors
// the badge's exactly: reconciled outranks cleared (a reconciled row is also
// cleared), and a transfer leg is never "needs review" because nothing reviews a
// transfer leg.
//
// An em dash rather than an empty cell for the states with no word: a blank cell in
// a status column reads as "we do not know", which is a different claim from "there
// is nothing to say here".
func rowStatusWord(p txnFrameRowProps) string {
	switch {
	case p.Reconciled:
		return uistate.T("acctxn.legendReconciled")
	case p.Cleared:
		return uistate.T("acctxn.legendCleared")
	case !p.Reviewed && !p.IsTransfer:
		return uistate.T("acctxn.legendNeedsReview")
	case p.Reviewed:
		return uistate.T("transactions.statusReviewed")
	}
	return "—"
}

// statusToneClass tints the word.
//
// Colour is the SECOND signal here, never the only one — the word carries the
// meaning and the tint only speeds the scan, which is the order C578 asks for.
// Only the state that wants attention is tinted at all; "Cleared" is the resting
// state of a healthy ledger and colouring every settled row would leave the one row
// that needs a person with nothing to distinguish it.
func statusToneClass(p txnFrameRowProps) string {
	if !p.Reconciled && !p.Cleared && !p.Reviewed && !p.IsTransfer {
		return "txn-status-attention"
	}
	return "text-dim"
}
