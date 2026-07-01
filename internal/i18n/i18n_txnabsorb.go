// SPDX-License-Identifier: MIT

package i18n

// txnAbsorbKeys holds the English strings added for the /transactions Import and
// Review-duplicates entry points (FEATURE_MAP §5.3 absorb). Kept separate from
// en.go so this change can be reviewed and reverted without touching the main
// catalog. Merged via init() per the project's init-merge pattern.
var txnAbsorbKeys = Catalog{
	// Import panel toggle in the Transactions header.
	"transactions.importBtn":      "Import",
	"transactions.importBtnClose": "Close import",
	// Duplicates panel toggle in the Transactions header.
	"transactions.dupReviewBtn":   "Review duplicates",
	"transactions.dupReviewClose": "Close review",
	"transactions.dupReviewBadge": "Review %s",
}

func init() {
	for k, v := range txnAbsorbKeys {
		english[k] = v
	}
}
