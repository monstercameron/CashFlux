// Package prefs models user display preferences that persist locally across
// reloads (week start and date format), independent of the dataset. The display
// logic lives here as pure Go so it is unit-tested on native Go; the wasm layer
// only handles localStorage persistence and the settings UI.
package prefs

import "time"

// WeekStart is the first day of the week for calendars and weekly periods.
type WeekStart string

// The supported week-start choices.
const (
	WeekSunday WeekStart = "sunday"
	WeekMonday WeekStart = "monday"
)

// DateStyle is how dates are rendered to the user.
type DateStyle string

// The supported date styles.
const (
	DateISO  DateStyle = "iso"  // 2006-01-02
	DateUS   DateStyle = "us"   // 01/02/2006
	DateEU   DateStyle = "eu"   // 02/01/2006
	DateLong DateStyle = "long" // Jan 2, 2006
)

// Prefs holds the user's display preferences.
type Prefs struct {
	WeekStart WeekStart `json:"weekStart"`
	DateStyle DateStyle `json:"dateStyle"`
}

// Default returns the out-of-the-box preferences (Sunday week start, ISO dates).
func Default() Prefs {
	return Prefs{WeekStart: WeekSunday, DateStyle: DateISO}
}

// Normalize fills any blank or unrecognized field with its default, so partial or
// older persisted data is always usable.
func (p Prefs) Normalize() Prefs {
	switch p.WeekStart {
	case WeekSunday, WeekMonday:
	default:
		p.WeekStart = WeekSunday
	}
	switch p.DateStyle {
	case DateISO, DateUS, DateEU, DateLong:
	default:
		p.DateStyle = DateISO
	}
	return p
}

// dateLayout maps a date style to its Go reference layout.
func dateLayout(s DateStyle) string {
	switch s {
	case DateUS:
		return "01/02/2006"
	case DateEU:
		return "02/01/2006"
	case DateLong:
		return "Jan 2, 2006"
	default:
		return "2006-01-02"
	}
}

// FormatDate renders a time using the preferred date style.
func (p Prefs) FormatDate(t time.Time) string {
	return t.Format(dateLayout(p.Normalize().DateStyle))
}

// WeekStartWeekday returns the configured first day of the week as a time.Weekday.
func (p Prefs) WeekStartWeekday() time.Weekday {
	if p.Normalize().WeekStart == WeekMonday {
		return time.Monday
	}
	return time.Sunday
}

// WeekStartOf returns the start of the week (at 00:00 in t's location) that
// contains t, honoring the configured first day of the week.
func (p Prefs) WeekStartOf(t time.Time) time.Time {
	y, m, d := t.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	offset := (int(day.Weekday()) - int(p.WeekStartWeekday()) + 7) % 7
	return day.AddDate(0, 0, -offset)
}
