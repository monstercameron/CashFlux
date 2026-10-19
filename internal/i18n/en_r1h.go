// SPDX-License-Identifier: MIT

package i18n

// r1hKeys contains copy for scheduled future-period rows and the transactions
// calendar's density legend and weekly totals.
var r1hKeys = Catalog{
	"transactions.noMatchFutureListLabel": "Scheduled items in this period",
	"transactions.scheduledStatus":        "Scheduled",
	"transactions.calDotsLegend":          "Each dot represents about five transactions.",
	"transactions.calWeekTotal":           "Week total",
	"transactions.calWeekTotalAria":       "Week net %s across %d transactions",
	"transactions.calWeekCount":           "%d transactions",
}

func init() {
	for key, value := range r1hKeys {
		english[key] = value
	}
}
