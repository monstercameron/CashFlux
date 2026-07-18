// SPDX-License-Identifier: MIT

package i18n

// lane1Keys holds English strings added by the reports/trust remediation lane
// (QA CF-01/UX-03 scope honesty, report modal standardization, provenance).
// Merged via init so this file never touches the shared en.go.
var lane1Keys = Catalog{
	// %s = plain-English scope parts, %s = the review window line.
	"rpta.scopeShowing": "Showing %s only · %s — sections tagged “Household-wide” ignore this scope",
	"rpta.scopeReset":   "Reset scope",
	// %s = the snapshot's period label (e.g. "Jul 2026").
	"reports.snapModalTitle": "Snapshot — %s",
	// #54 financial audit trail: cause badges + the downstream-figures line.
	"activity.causeRule":   "Rule",
	"activity.causeImport": "Import",
	"activity.causeAI":     "AI assistant",
	// %s = " · "-joined figure names (e.g. "account balance · budget progress").
	"activity.recalc": "Recalculated: %s",
	// #56 number provenance: the masthead figures' click-to-explain popovers.
	"rpta.provHint": "How is this number built?",
	// %s = "N income transactions" / "N spending transactions", %s = "N accounts".
	"rpta.provCounted": "Built from %s across %s.",
	// %s = "N transactions", %s = "N accounts".
	"rpta.provKept": "Income minus spending — %s across %s.",
	// %s = "N transfers".
	"rpta.provTransfers": "%s ignored — money moving between your accounts is never income or spending.",
	// %s = "N transactions".
	"rpta.provExcluded": "%s marked \"exclude from reports\" are not counted.",
	// %s = "N accounts" in scope.
	"rpta.provNWAccounts": "Assets minus debts across %s.",
	// %s = "N transactions", %s = the window's last month (e.g. "Jul 2026").
	"rpta.provNWTxns": "Balances built from %s recorded through %s.",
}

func init() {
	for k, v := range lane1Keys {
		english[k] = v
	}
}
