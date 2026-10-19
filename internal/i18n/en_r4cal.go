// SPDX-License-Identifier: MIT

package i18n

var r4calKeys = Catalog{
	"transactions.calDotsLegend":                 "More dots mean more transactions.",
	"transactions.calWeekTotal":                  "Week total",
	"transactions.calDueFilterNote":              "Due items follow account, category, and member filters. Other filters apply to transactions only.",
	"transactions.calScheduledDue":               "Due %s %s",
	"transactions.calScheduledDueAria":           "%s is due on this day for %s.",
	"transactions.calNotPostedStatus":            "%s not posted",
	"transactions.calNotPostedAria":              "%s was due on this day for %s, but it was not posted.",
	"transactions.calDueStatus":                  "Due %s",
	"transactions.calScheduledAmountUnavailable": "Amount unavailable",
}

func init() {
	for key, value := range r4calKeys {
		english[key] = value
	}
}
