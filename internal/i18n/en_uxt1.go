// SPDX-License-Identifier: MIT

package i18n

// uxt1Keys contains the transaction period summary copy for UXT1.
var uxt1Keys = Catalog{
	"uxt1.accessibleSummary": "%s. %s. Spent %s. Income %s. Net %s",
	"uxt1.categoryBreakdown": "Spending by category",
	"uxt1.categoryFilter":    "Filter by %s, %s",
	"uxt1.uncategorized":     "Uncategorized",
}

func init() {
	for key, value := range uxt1Keys {
		english[key] = value
	}
}
