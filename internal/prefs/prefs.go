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

// Theme is the color theme of the app.
type Theme string

// The supported themes. ThemeSystem follows the OS preference.
const (
	ThemeDark   Theme = "dark"
	ThemeLight  Theme = "light"
	ThemeSystem Theme = "system"
)

// defaultAccent is the out-of-the-box accent color: a seagreen that clears WCAG
// AA for UI/large elements (3:1) against BOTH the dark and light theme surfaces
// (dark 4.09:1, light 3.63:1), unlike the original lighter mint #54b884 which
// failed on light (~2.1:1). Accent drives the focus ring and large strokes, so it
// must pass the 3:1 threshold on whichever surface the user runs (B15).
const defaultAccent = "#2e8b57"

// Display-scale bounds (whole percent). 100 is the unscaled default. The upper
// bound reaches 200 so the scale doubles as an accessibility text-resize control
// (WCAG 2.1 SC 1.4.4 "Resize text" — text must reach 200% without loss of
// content); the responsive layout (C10/C19) reflows at the resulting effective
// width instead of overflowing (C26).
const (
	ScaleMin     = 70
	ScaleMax     = 200
	ScaleDefault = 100
)

// Prefs holds the user's display preferences.
type Prefs struct {
	WeekStart WeekStart `json:"weekStart"`
	DateStyle DateStyle `json:"dateStyle"`
	Theme     Theme     `json:"theme"`
	Accent    string    `json:"accent"`          // hex color, e.g. "#54b884"
	Compact   bool      `json:"compact"`         // denser layout
	Scale     int       `json:"scale,omitempty"` // UI zoom percent (70..200); 0 means default 100
	// RememberAIKey opts into persisting the OpenAI key on this device across
	// reloads (off by default — the key is otherwise session-only). When on, the
	// key is written to its own localStorage entry, separate from the dataset.
	RememberAIKey bool `json:"rememberAiKey,omitempty"`
}

// Default returns the out-of-the-box preferences (Sunday week start, ISO dates,
// dark theme, green accent, comfortable density, 100% scale).
func Default() Prefs {
	return Prefs{WeekStart: WeekSunday, DateStyle: DateISO, Theme: ThemeDark, Accent: defaultAccent, Scale: ScaleDefault}
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
	switch p.Theme {
	case ThemeDark, ThemeLight, ThemeSystem:
	default:
		p.Theme = ThemeDark
	}
	if !isHexColor(p.Accent) {
		p.Accent = defaultAccent
	}
	switch {
	case p.Scale == 0:
		p.Scale = ScaleDefault
	case p.Scale < ScaleMin:
		p.Scale = ScaleMin
	case p.Scale > ScaleMax:
		p.Scale = ScaleMax
	}
	return p
}

// ScaleFraction returns the display scale as a CSS zoom multiplier (e.g. 1.1 for
// 110%), normalized into the supported range.
func (p Prefs) ScaleFraction() float64 {
	return float64(p.Normalize().Scale) / 100
}

// isHexColor reports whether s is a "#rgb" or "#rrggbb" hex color.
func isHexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 {
		return false
	}
	if s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
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
