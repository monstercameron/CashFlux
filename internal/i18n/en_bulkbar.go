// SPDX-License-Identifier: MIT

package i18n

// bulkBarKeys holds the SHORT verb labels for the transactions bulk-action bar. The
// full descriptive sentences (e.g. "Mark the selected transactions cleared") stay as
// each button's tooltip/aria-label; these short labels are what the button shows, so
// the bar stays a compact single row instead of a stack of full-sentence buttons.
// Merged via init so this file does not touch en.go.
var bulkBarKeys = Catalog{
	"transactions.bulkSelectedShort":     "%d selected",
	"transactions.bulkCatShort":          "Categorize",
	"transactions.bulkAssignShort":       "Assign",
	"transactions.bulkClearedShort":      "Mark cleared",
	"transactions.bulkUnclearedShort":    "Mark not cleared",
	"transactions.bulkGroupShort":        "Group as one purchase",
	"transactions.bulkExportShort":       "Export",
	"transactions.bulkDeleteShort":       "Delete",
	"transactions.bulkDeleteConfirmOne":  "Delete %d transaction? You can undo this.",
	"transactions.bulkDeleteConfirmMany": "Delete %d transactions? You can undo this.",
}

func init() {
	for k, v := range bulkBarKeys {
		english[k] = v
	}
}
