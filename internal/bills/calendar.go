package bills

import "time"

// CalendarDay is one cell of a month grid: its date, whether it belongs to the
// displayed month (leading/trailing cells from adjacent months are false), and
// the bills due that day.
type CalendarDay struct {
	Date    time.Time
	InMonth bool
	Bills   []Bill
}

// MonthCalendar lays out the given month as a grid of whole weeks, each starting
// on weekStart, with every bill placed on its due day. The first and last weeks
// are padded with the adjacent months' days (InMonth=false, never carrying
// bills) so each row has exactly seven cells. Only bills whose due date falls on
// a grid day are placed; a bill outside the visible range is simply omitted.
func MonthCalendar(bs []Bill, year int, month time.Month, weekStart time.Weekday) [][]CalendarDay {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	offset := (int(first.Weekday()) - int(weekStart) + 7) % 7
	gridStart := first.AddDate(0, 0, -offset)

	dim := daysInMonth(year, month)
	weeks := (offset + dim + 6) / 7 // ceil to whole weeks

	// Index bills by their due day so each cell is a quick lookup.
	byDay := map[string][]Bill{}
	for _, b := range bs {
		byDay[dayKey(b.DueDate)] = append(byDay[dayKey(b.DueDate)], b)
	}

	grid := make([][]CalendarDay, 0, weeks)
	day := gridStart
	for w := 0; w < weeks; w++ {
		row := make([]CalendarDay, 7)
		for i := 0; i < 7; i++ {
			row[i] = CalendarDay{
				Date:    day,
				InMonth: day.Month() == month && day.Year() == year,
				Bills:   byDay[dayKey(day)],
			}
			day = day.AddDate(0, 0, 1)
		}
		grid = append(grid, row)
	}
	return grid
}

// dayKey is the date-only key used to bucket bills by due day.
func dayKey(t time.Time) string {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}
