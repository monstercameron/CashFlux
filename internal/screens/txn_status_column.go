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
	// NEEDS REVIEW OUTRANKS SETTLEMENT. These are two different axes — whether the
	// bank has settled the charge, and whether a person has looked at it — and a
	// charge can be both cleared and unreviewed. Ranking cleared first meant the
	// seeded known-bad control, which is cleared AND flagged, read simply "Cleared"
	// in the ledger with no hint it was sitting in the review queue: the column said
	// the row was fine while "Review inbox (249)" was counting it (C601).
	//
	// The one that asks for a person wins. Settlement is bookkeeping the ledger
	// records; review is work the ledger is asking for, and a status column that
	// hides the ask is worse than the bare dot it replaced. The settled state is not
	// lost — statusDetail puts it in the cell's tooltip and accessible name.
	switch {
	case !p.Reviewed && !p.IsTransfer:
		return uistate.T("acctxn.legendNeedsReview")
	case p.Reconciled:
		return uistate.T("acctxn.legendReconciled")
	case p.Cleared:
		return uistate.T("acctxn.legendCleared")
	case p.Reviewed:
		return uistate.T("transactions.statusReviewed")
	}
	return "—"
}

// statusDetail is the cell's full state in one sentence, for its tooltip and
// accessible name — so the axis the word had to drop is still available.
//
// A row that is both settled and unreviewed reads "Needs review" and hovers
// "Needs review · cleared". Without this the reordering above would trade one
// hidden fact for another.
func statusDetail(p txnFrameRowProps) string {
	word := rowStatusWord(p)
	if p.Reviewed || p.IsTransfer {
		return word
	}
	switch {
	case p.Reconciled:
		return uistate.T("transactions.statusAlso", word, uistate.T("acctxn.legendReconciled"))
	case p.Cleared:
		return uistate.T("transactions.statusAlso", word, uistate.T("acctxn.legendCleared"))
	}
	return word
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
