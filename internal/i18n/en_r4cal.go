// SPDX-License-Identifier: MIT

package i18n

var r4calKeys = Catalog{
	"transactions.calDotsLegend":                 "More dots mean more transactions.",
	"transactions.calWeekTotal":                  "Week total",
	"transactions.calScheduledDue":               "%s due · %s",
	"transactions.calScheduledDueAria":           "%s is scheduled for this day. Amount: %s.",
	"transactions.calNotPosted":                  "Not posted yet · %s · %s",
	"transactions.calNotPostedAria":              "Not posted yet. %s was due on this day for %s.",
	"transactions.calScheduledAmountUnavailable": "Amount unavailable",
}

func init() {
	for key, value := range r4calKeys {
		english[key] = value
	}
}
