// SPDX-License-Identifier: MIT

package i18n

// lane1Keys holds English strings added by the reports/trust remediation lane
// (QA CF-01/UX-03 scope honesty, report modal standardization, provenance).
// Merged via init so this file never touches the shared en.go.
var lane1Keys = Catalog{
	// %s = plain-English scope parts, %s = the review window line.
	"rpta.scopeShowing": "Showing %s only · %s — health score and Where you stand stay household-wide",
	"rpta.scopeReset":   "Reset scope",
	// %s = the snapshot's period label (e.g. "Jul 2026").
	"reports.snapModalTitle": "Snapshot — %s",
	// #54 financial audit trail: cause badges + the downstream-figures line.
	"activity.causeRule":   "Rule",
	"activity.causeImport": "Import",
	"activity.causeAI":     "AI assistant",
	// %s = " · "-joined figure names (e.g. "account balance · budget progress").
	"activity.recalc": "Recalculated: %s",
}

func init() {
	for k, v := range lane1Keys {
		english[k] = v
	}
}
