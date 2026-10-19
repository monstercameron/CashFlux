// SPDX-License-Identifier: MIT

package i18n

// TV8E transaction copy corrections that need a catalog override.
var tv8eKeys = Catalog{
	// The unusual-charge insight passes both a signal name and an amount.
	"smart.u1.title": "%s charge: %s",

	// These variants let the transaction screen use grammatical singular and
	// plural copy without embedding English plural logic in its view.
	"transactions.bulkCatAppliedOne":    "Filed under %s for 1 transaction",
	"transactions.bulkCatAppliedOther":  "Filed under %s for %d transactions",
	"transactions.clearAllFiltersOne":   "Clear filter",
	"transactions.clearAllFiltersOther": "Clear %d filters",
}

func init() {
	for key, value := range tv8eKeys {
		english[key] = value
	}
}
