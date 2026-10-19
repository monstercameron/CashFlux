// SPDX-License-Identifier: MIT

package i18n

// r1dKeys contains the period selector and custom-range labels introduced by R1D.
var r1dKeys = Catalog{
	"txnperiod.periodLabel":       "Period",
	"txnperiod.lastMonth":         "Last month",
	"txnperiod.lastThreeMonths":   "Last 3 months",
	"txnperiod.thisYear":          "This year",
	"txnperiod.customRangeChoice": "Custom range",
	"txnperiod.from":              "From",
	"txnperiod.to":                "To",
}

func init() {
	for key, value := range r1dKeys {
		english[key] = value
	}
}
