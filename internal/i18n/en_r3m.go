// SPDX-License-Identifier: MIT

package i18n

// r3mKeys describe calendar totals in language that matches their signed display.
var r3mKeys = Catalog{
	"transactions.calDayEmpty":      "Day %s, no transactions",
	"transactions.calDaySpent":      "Day %s, spent %s",
	"transactions.calDayNet":        "Day %s, net %s",
	"transactions.calDayZero":       "Day %s, net %s",
	"transactions.calWeekSpentAria": "Week: spent %s across %d transactions",
	"transactions.calWeekNetAria":   "Week: net %s across %d transactions",
	"transactions.calWeekZeroAria":  "Week: net %s across %d transactions",
}

func init() {
	for key, value := range r3mKeys {
		english[key] = value
	}
}
