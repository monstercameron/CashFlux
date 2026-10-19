// SPDX-License-Identifier: MIT

package i18n

// v5sumKeys contains copy for the compact, proportional transaction summary.
var v5sumKeys = Catalog{
	"v5sum.other":           "Other",
	"v5sum.more":            "+%d more",
	"v5sum.filteredSummary": "%s. %s. Spent %s. Spending by category follows.",
}

func init() {
	for key, value := range v5sumKeys {
		english[key] = value
	}
}
