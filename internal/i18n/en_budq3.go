// SPDX-License-Identifier: MIT

package i18n

// budgetsQ3MonthKeys are the annual budget grid's localized month labels.
var budgetsQ3MonthKeys = Catalog{
	"budgets.month.jan":       "Jan",
	"budgets.month.feb":       "Feb",
	"budgets.month.mar":       "Mar",
	"budgets.month.apr":       "Apr",
	"budgets.month.may":       "May",
	"budgets.month.jun":       "Jun",
	"budgets.month.jul":       "Jul",
	"budgets.month.aug":       "Aug",
	"budgets.month.sep":       "Sep",
	"budgets.month.oct":       "Oct",
	"budgets.month.nov":       "Nov",
	"budgets.month.dec":       "Dec",
	"budgets.month.january":   "January",
	"budgets.month.february":  "February",
	"budgets.month.march":     "March",
	"budgets.month.april":     "April",
	"budgets.month.mayFull":   "May",
	"budgets.month.june":      "June",
	"budgets.month.july":      "July",
	"budgets.month.august":    "August",
	"budgets.month.september": "September",
	"budgets.month.october":   "October",
	"budgets.month.november":  "November",
	"budgets.month.december":  "December",
}

func init() {
	for key, value := range budgetsQ3MonthKeys {
		english[key] = value
	}
}
