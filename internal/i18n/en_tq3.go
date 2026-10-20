// SPDX-License-Identifier: MIT

package i18n

var tq3Keys = Catalog{
	"transactions.tq3.weekday.sun":          "Sun",
	"transactions.tq3.weekday.mon":          "Mon",
	"transactions.tq3.weekday.tue":          "Tue",
	"transactions.tq3.weekday.wed":          "Wed",
	"transactions.tq3.weekday.thu":          "Thu",
	"transactions.tq3.weekday.fri":          "Fri",
	"transactions.tq3.weekday.sat":          "Sat",
	"transactions.tq3.weekday.sun.full":     "Sunday",
	"transactions.tq3.weekday.mon.full":     "Monday",
	"transactions.tq3.weekday.tue.full":     "Tuesday",
	"transactions.tq3.weekday.wed.full":     "Wednesday",
	"transactions.tq3.weekday.thu.full":     "Thursday",
	"transactions.tq3.weekday.fri.full":     "Friday",
	"transactions.tq3.weekday.sat.full":     "Saturday",
	"transactions.tq3.dayEmpty":             "Day %s: no transactions.",
	"transactions.tq3.dayCashflow":          "Day %s: spent %s, income %s, transferred %s, %s.",
	"transactions.tq3.dayOne":               "1 transaction",
	"transactions.tq3.dayMany":              "%d transactions",
	"transactions.tq3.monthFxMissing":       "%d transaction amounts could not be converted to the base currency and are not included in these calendar totals.",
	"transactions.tq3.weekSummary":          "Week of %s: spent %s, income %s, transferred %s, %s.%s",
	"transactions.tq3.weekOneSummary":       "1 transaction",
	"transactions.tq3.weekManySummary":      "%d transactions",
	"transactions.tq3.weekSummaryScheduled": " %d due.",
	"transactions.tq3.weekScheduled":        "%d due",
	"transactions.tq3.weekCount":            "%d transactions",
	"transactions.tq3.weekOne":              "1 transaction",
	"transactions.tq3.weekEmpty":            "Nothing yet",
	"transactions.tq3.weekMoved":            "Transferred %s",
	"transactions.tq3.dayMoved":             "Transferred %s",
	"transactions.calMoreDue":               "+%d more due",
	"transactions.calScrollHint":            "Scroll to see all days",
	"transactions.calOtherDue":              "Other due items (%d)",
	"transactions.calDayNumber":             "Day %d",
}

func init() {
	for key, value := range tq3Keys {
		english[key] = value
	}
}
