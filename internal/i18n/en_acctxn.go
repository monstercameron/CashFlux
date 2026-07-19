// SPDX-License-Identifier: MIT

package i18n

// acctxnKeys holds the English strings added by the 2026-07-19 Accounts +
// Transactions UX refinement pass: the "Account settings" rename of the accounts
// toolbar's management menu, the plain-English meaning behind the global
// "Mark all updated" trust action, and the transactions ledger's status-glyph
// legend. Merged via init so the shared en.go is never touched by this lane.
var acctxnKeys = Catalog{
	// Accounts — the toolbar management menu, renamed from the vague "Manage" to
	// "Account settings" so its contents (groups, institutions, sweep rules,
	// exchange rates) read as configuration rather than an unspecified verb.
	"acctxn.acctSettingsMenu":      "Account settings",
	"acctxn.acctSettingsMenuTitle": "Account settings — groups, institutions, sweep rules, and exchange rates",

	// Accounts — the one-line meaning shown as the "Mark all updated" tooltip and
	// appended to its confirm dialog, so a powerful global action explains itself.
	"acctxn.markAllMeaning": "This confirms every stale balance is current as of today.",

	// Transactions — the ledger's row status-glyph legend, so the compact ✓✓ / ✓ / •
	// markers are decoded in plain English instead of relying on shape or color.
	"acctxn.legendLabel":       "Status:",
	"acctxn.legendAria":        "What the row status marks mean",
	"acctxn.legendReconciled":  "Reconciled",
	"acctxn.legendCleared":     "Cleared",
	"acctxn.legendNeedsReview": "Needs review",
}

func init() {
	for k, v := range acctxnKeys {
		english[k] = v
	}
}
