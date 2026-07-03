// SPDX-License-Identifier: MIT

package i18n

// reportSurfaceKeys holds the English strings for the redesigned /reports bento
// surface (hero tile, toolbar, scope filter, export menu, report metrics).
// Merged via init so this file does not touch en.go; mirrors the
// en_recurringsurface.go pattern.
var reportSurfaceKeys = Catalog{
	// Hero
	"reports.heroTitle": "This period",

	// Toolbar
	"reports.scope":        "Scope",
	"reports.scopeCount":   "Scope (%d)",
	"reports.scopeHint":    "Filter every figure by institution, owner, account type, or a saved view",
	"reports.metricsShow":  "Report metrics",
	"reports.metricsHide":  "Hide metrics",
	"reports.metricsTitle": "Show every report figure as a live formula variable",
	"reports.exportCsv":    "Export CSV",
	"reports.exportTitle":  "Download this report's tables as CSV files, or save the page as a PDF",

	// Sections
	"reports.moneyFlow":           "Money flow",
	"reports.zeroedSummary":       "%d categories had spending last period but none this period",
	"reports.savingsTrendHint":    "Savings rate over the last %d periods.",
	"reports.customFieldUnvalued": "Nothing this period has a %s value yet — set it on transactions in the ledger to see this breakdown.",

	// Metrics tile
	"reports.formulaHint": "These report figures are live report_* engine variables — drop any of them into a formula or a dashboard widget.",
}

func init() {
	for k, v := range reportSurfaceKeys {
		english[k] = v
	}
}
