// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
)

// FlexTargetForWindow scales a monthly flex target by covered calendar days in
// each month. Each segment rounds half away from zero before checked addition.
// Endpoints use their wall-calendar dates; empty/reversed ranges retain zero.
func FlexTargetForWindow(monthly int64, start, end time.Time, base string) (money.Money, error) {
	total := money.Zero(base)
	start, end = dateutil.CalendarDayUTC(start), dateutil.CalendarDayUTC(end)
	for cursor := start; cursor.Before(end); {
		monthStart := dateutil.MonthStart(cursor)
		nextMonth := dateutil.AddMonths(monthStart, 1)
		segmentEnd := end
		if segmentEnd.After(nextMonth) {
			segmentEnd = nextMonth
		}
		days := int64(dateutil.DaysBetween(cursor, segmentEnd))
		monthDays := int64(dateutil.DaysBetween(monthStart, nextMonth))
		segment, err := money.New(monthly, base).MulDiv(days, monthDays, money.RoundHalfAwayFromZero)
		if err != nil {
			return money.Money{}, fmt.Errorf("scale flex target for %s: %w", monthStart.Format("2006-01"), err)
		}
		total, err = total.Add(segment)
		if err != nil {
			return money.Money{}, fmt.Errorf("sum flex target window: %w", err)
		}
		cursor = segmentEnd
	}
	return total, nil
}
