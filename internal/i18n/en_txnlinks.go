// SPDX-License-Identifier: MIT

package i18n

// txnLinksKeys holds English strings for the transaction-link features (XC1
// order grouping and XC2 refund pairing). Kept in its own file (not en.go) so
// this change does not collide with concurrent edits to the main catalog.
var txnLinksKeys = Catalog{
	// XC1 — order grouping.
	"txnlinks.groupAction":     "Group as one purchase",
	"txnlinks.groupNeedTwo":    "Select at least two transactions to group them as one purchase.",
	"txnlinks.grouped":         "Grouped %d transactions as one purchase.",
	"txnlinks.groupErr":        "Couldn't group those transactions: %s",
	"txnlinks.ungroupAction":   "Ungroup",
	"txnlinks.ungrouped":       "Ungrouped — the transactions are separate again.",
	"txnlinks.groupBadge":      "Part of a %d-charge order",
	"txnlinks.groupBadgeTitle": "This charge is one of %d in a single order (%s total).",
	"txnlinks.orderTotalLabel": "Order total",
	"txnlinks.balanced":        "Balanced — charges match the order total.",
	"txnlinks.remainder":       "%s left to match the order total.",
	"txnlinks.overBy":          "%s more than the order total.",

	// XC2 — refund pairing.
	"txnlinks.pairAction":    "Pair as refund of…",
	"txnlinks.pairTitle":     "Pair this refund with its purchase",
	"txnlinks.pairIntro":     "Match this refund to the purchase it returns. Budgets and reports will net it in the purchase's month; the ledger keeps both.",
	"txnlinks.pairNoneFound": "No matching purchase found — same payee, at least this amount, within the last 90 days.",
	"txnlinks.pairChoose":    "Choose the original purchase",
	"txnlinks.pairConfirm":   "Pair as refund",
	"txnlinks.paired":        "Paired — this refund now nets against its purchase.",
	"txnlinks.pairErr":       "Couldn't pair that refund: %s",
	"txnlinks.pairMissing":   "That transaction is no longer here.",
	"txnlinks.unpairAction":  "Remove refund pairing",
	"txnlinks.unpaired":      "Removed the refund pairing.",
	"txnlinks.refundBadge":   "Refund of a purchase",
	"txnlinks.refundedBadge": "Refunded",
	"txnlinks.pairNetLabel":  "Nets %s in %s",
	"txnlinks.notARefund":    "Only a positive (money-in) transaction can be paired as a refund.",
}

func init() {
	for k, v := range txnLinksKeys {
		english[k] = v
	}
}
