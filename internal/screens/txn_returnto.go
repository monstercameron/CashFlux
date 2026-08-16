// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// txnLeaveFor records the breadcrumb for a cross-page correction that starts on
// the ledger, naming the working set the user is stepping away from (C581).
//
// Every departure from the ledger goes through here rather than each caller
// hand-building a label, so "Rules from a row", "Rules from the toolbar" and "the
// undo bar's Activity link" all describe the same thing the same way — and adding
// a new destination is one call, not a new dialect.
func txnLeaveFor(to string) {
	uistate.SetReturnTo(uistate.ReturnTo{
		From:  "/transactions",
		To:    to,
		Label: txnWorkingSetLabel(uistate.CurrentTxFilter()),
	})
}

// txnWorkingSetLabel describes the ledger's current working set in a few words.
//
// The point is to name the thing the user will come back TO, so the crumb reads
// "Back to Transactions, filtered to "coffee"" rather than a bare "Back". The
// search text is the most recognisable handle when there is one — it is what the
// user typed — and a filter count is the honest fallback: listing five chips in a
// breadcrumb would be a worse summary than saying how many there are.
func txnWorkingSetLabel(c txnfilter.Criteria) string {
	if q := strings.TrimSpace(c.Text); q != "" {
		return uistate.T("returnTo.labelSearch", q)
	}
	if n := len(c.ActiveFilters()); n > 0 {
		return uistate.T("returnTo.labelFiltered", plural(n, "filter"))
	}
	return uistate.T("returnTo.labelPlain")
}
