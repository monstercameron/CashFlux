// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "github.com/monstercameron/CashFlux/internal/uistate"

// rowStatusWord returns only the actionable status. Settled and reviewed rows
// have no status text; their cleared state is shown by the row's check glyph.
func rowStatusWord(p txnFrameRowProps) string {
	if p.Queued && !p.IsTransfer {
		return uistate.T("acctxn.legendNeedsReview")
	}
	return ""
}

// statusDetail supplies the accessible label and tooltip for a status cell.
func statusDetail(p txnFrameRowProps) string {
	return rowStatusWord(p)
}

// badgeStateLabel appends the settlement axis to a word that dropped it, so the
// cell's tooltip and the row glyph's accessible name both carry the whole state.
//
// C640: the gate used to be `p.Reviewed || p.IsTransfer`, which asked the wrong
// question. Reviewed and Queued are not mutually exclusive — re-flagging an
// already-reviewed charge (adding the needs-review tag in the Tags field, which
// nothing filters) leaves Reviewed true AND Queued true. rowStatusWord then
// correctly says "Needs review", but the old gate saw Reviewed and returned
// early, so the cell stopped mentioning that the charge was also cleared. The
// question that matters is whether the WORD is a review-axis word, because those
// are the only ones that leave settlement unsaid.
func badgeStateLabel(p txnFrameRowProps, word string) string {
	saidReview := (p.Queued || !p.Reviewed) && !p.IsTransfer
	if !saidReview {
		// A settlement word already states settlement, and a transfer has no
		// review axis to have dropped.
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
	// C618: reserve the attention tint for a row the Review inbox is actually
	// holding. "Not confirmed" is a softer state — the charge is filed, nobody has
	// signed it off — and tinting it the same colour would put an alarm on rows
	// that have no queue card to answer it, which is the noise this ticket is about.
	if p.Queued && !p.IsTransfer {
		return "txn-status-attention"
	}
	return ""
}
