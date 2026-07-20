// SPDX-License-Identifier: MIT

package i18n

// whatChangedKeys holds English strings for the dashboard "What changed since
// your last visit" card (E-DB / E1 attribution experiment, 2026-07-19). Merged
// via init so this file never touches the shared en.go.
var whatChangedKeys = Catalog{
	"dashboard.wcTitle":            "What changed",
	"dashboard.wcSince":            "since %s",
	"dashboard.wcAria":             "What changed since your last visit",
	"dashboard.wcGotIt":            "Got it",
	"dashboard.wcGotItTitle":       "Got it — start the next “what changed” from now",
	"dashboard.wcView":             "View",
	"dashboard.wcViewAccountTitle": "Open %s on the Accounts page",
	"dashboard.wcViewTxnsTitle":    "Open the Transactions page",

	// Row leads, one per finding kind.
	"dashboard.wcLeadNet":      "Net worth change",
	"dashboard.wcLeadCategory": "Top spending — %s",
	"dashboard.wcLeadIncome":   "Income landed",
	"dashboard.wcLeadLarge":    "Large expense — %s",
	"dashboard.wcLeadNew":      "New merchant — %s",
	"dashboard.wcUncategorized": "Uncategorized",

	// "Why" decomposition labels (joined with the amounts they explain).
	"dashboard.wcPartFlow":  "cash flow %s",
	"dashboard.wcPartAdj":   "balance updates %s",
	"dashboard.wcPartOther": "other changes %s",

	// Counts.
	"dashboard.wcTxnCount":    "%d transactions",
	"dashboard.wcTxnCountOne": "1 transaction",
	"dashboard.wcDeposits":    "%d deposits",
	"dashboard.wcDepositOne":  "1 deposit",
	"dashboard.wcNewMore":     "+%d more new merchants",
}

func init() {
	for k, v := range whatChangedKeys {
		english[k] = v
	}
}
