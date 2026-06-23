package period

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
)

// Preset window constructors for the resolution control's quick picks (B10).
// All take an explicit now so they stay pure and testable.

// Previous returns the single period immediately before the one containing now
// (last week / month / quarter), at the given resolution.
func Previous(r Resolution, now time.Time, weekStart time.Weekday) Window {
	return NewWindow(r, now, weekStart).Shift(-1)
}

// YearToDate returns a month-resolution window spanning January through the
// month containing now, in now's year.
func YearToDate(now time.Time, weekStart time.Weekday) Window {
	jan := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, now.Location())
	return Window{
		Res:       Month,
		From:      jan,
		To:        dateutil.MonthStart(now),
		WeekStart: weekStart,
	}
}

// PriorYear returns a Year-resolution window covering the full calendar year
// immediately before the one containing now (e.g. if now is in 2026 it returns
// the 2025 window). This is the primary quick-pick for tax-prep workflows
// (L58) where the user needs the complete prior year in one tap.
func PriorYear(now time.Time, weekStart time.Weekday) Window {
	return NewWindow(Year, now, weekStart).Shift(-1)
}
