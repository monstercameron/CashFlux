// SPDX-License-Identifier: MIT

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// txnUpcomingSettlementDate reports whether a transaction date is inside the
// current month's settled window: from month start through the current day.
func txnUpcomingSettlementDate(date, monthStart, today time.Time) bool {
	date = dateutil.DayStart(date)
	monthStart = dateutil.DayStart(monthStart)
	today = dateutil.DayStart(today)
	return !date.Before(monthStart) && !date.After(today)
}
