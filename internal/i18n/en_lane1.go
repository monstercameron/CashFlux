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
}

func init() {
	for k, v := range lane1Keys {
		english[k] = v
	}
}
