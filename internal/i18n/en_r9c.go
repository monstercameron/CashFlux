// SPDX-License-Identifier: MIT

package i18n

var r9cKeys = Catalog{
	"r9c.weekSpent":          "Spent %s",
	"r9c.weekIncome":         "+%s in",
	"r9c.weekNothingYet":     "Nothing yet",
	"r9c.weekTransactions":   "%d transactions",
	"r9c.weekOneTransaction": "1 transaction",
	"r9c.weekAria":           "Week of %s: spent %s, %s.",
	"r9c.weekAriaIncome":     "Week of %s: spent %s, +%s in, %s.",
}

func init() {
	for key, value := range r9cKeys {
		english[key] = value
	}
}
