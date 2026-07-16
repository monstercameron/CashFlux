// SPDX-License-Identifier: MIT

package i18n

// bulkBarKeys holds the SHORT verb labels for the transactions bulk-action bar. The
// full descriptive sentences (e.g. "Mark the selected transactions cleared") stay as
// each button's tooltip/aria-label; these short labels are what the button shows, so
// the bar stays a compact single row instead of a stack of full-sentence buttons.
// Merged via init so this file does not touch en.go.
var bulkBarKeys = Catalog{
	"transactions.bulkSelectedShort":  "%d selected",
	"transactions.bulkCatShort":       "Categorize",
	"transactions.bulkAssignShort":    "Assign",
	"transactions.bulkClearedShort":   "Cleared",
	"transactions.bulkUnclearedShort": "Uncleared",
	"transactions.bulkGroupShort":     "Group",
	"transactions.bulkExportShort":    "Export",
	"transactions.bulkDeleteShort":    "Delete",
}

func init() {
	for k, v := range bulkBarKeys {
		english[k] = v
	}
}
