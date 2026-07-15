// SPDX-License-Identifier: MIT

package i18n

// ledgerViewKeys holds English strings for the transactions ledger view modes:
// the calendar month grid (TX8) and register running-balance mode (TX12). Kept in
// a separate file (not en.go) so this change does not touch the concurrent-WIP
// file. Registered into the catalog at init time.
var ledgerViewKeys = Catalog{
	// Calendar view mode (TX8).
	"transactions.calendarView": "Calendar",
	"transactions.calToday":     "Today",
	"transactions.calPrevMonth": "Previous month",
	"transactions.calNextMonth": "Next month",
	// Accessible label for a day cell: %s = day number, %s = the day's net amount
	// (may be empty for a day with no activity).
	"transactions.calDayLabel": "%s, net %s",

	// Register view mode (TX12).
	"transactions.registerView": "Register",
	"transactions.colBalance":   "Balance",
}

func init() {
	for k, v := range ledgerViewKeys {
		english[k] = v
	}
}
