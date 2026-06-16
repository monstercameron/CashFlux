package prefs

import (
	"testing"
	"time"
)

func TestFormatDate(t *testing.T) {
	d := time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		style DateStyle
		want  string
	}{
		{DateISO, "2026-06-05"},
		{DateUS, "06/05/2026"},
		{DateEU, "05/06/2026"},
		{DateLong, "Jun 5, 2026"},
		{DateStyle("bogus"), "2026-06-05"}, // falls back to ISO
	}
	for _, tt := range tests {
		p := Prefs{DateStyle: tt.style}
		if got := p.FormatDate(d); got != tt.want {
			t.Errorf("style %q: got %q, want %q", tt.style, got, tt.want)
		}
	}
}

func TestWeekStartWeekday(t *testing.T) {
	if (Prefs{WeekStart: WeekMonday}).WeekStartWeekday() != time.Monday {
		t.Error("Monday pref should map to time.Monday")
	}
	if (Prefs{WeekStart: WeekSunday}).WeekStartWeekday() != time.Sunday {
		t.Error("Sunday pref should map to time.Sunday")
	}
	if (Prefs{}).WeekStartWeekday() != time.Sunday {
		t.Error("blank week start should default to Sunday")
	}
}

func TestWeekStartOf(t *testing.T) {
	// Wednesday, 2026-06-10.
	wed := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	sun := Prefs{WeekStart: WeekSunday}.WeekStartOf(wed)
	if sun.Weekday() != time.Sunday || sun.Day() != 7 {
		t.Errorf("Sunday week start: got %v, want 2026-06-07", sun.Format("2006-01-02"))
	}
	mon := Prefs{WeekStart: WeekMonday}.WeekStartOf(wed)
	if mon.Weekday() != time.Monday || mon.Day() != 8 {
		t.Errorf("Monday week start: got %v, want 2026-06-08", mon.Format("2006-01-02"))
	}
	// Time-of-day is dropped.
	if sun.Hour() != 0 || sun.Minute() != 0 {
		t.Errorf("week start should be midnight, got %v", sun)
	}
}

func TestNormalize(t *testing.T) {
	got := Prefs{WeekStart: "x", DateStyle: "y"}.Normalize()
	if got != Default() {
		t.Errorf("bad values should normalize to default, got %+v", got)
	}
	keep := Prefs{WeekStart: WeekMonday, DateStyle: DateLong}
	if keep.Normalize() != keep {
		t.Errorf("valid values should be preserved, got %+v", keep.Normalize())
	}
}
