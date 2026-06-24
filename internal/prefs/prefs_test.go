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

func TestScaleNormalize(t *testing.T) {
	cases := map[int]int{
		0:   ScaleDefault, // unset → 100
		100: 100,
		70:  70,
		130: 130,
		200: 200,      // 200% is in range (accessibility text-resize, C26)
		50:  ScaleMin, // below range clamps up
		250: ScaleMax, // above range clamps down to 200
		90:  90,
	}
	for in, want := range cases {
		if got := (Prefs{Scale: in}).Normalize().Scale; got != want {
			t.Errorf("Normalize scale %d = %d, want %d", in, got, want)
		}
	}
	if f := (Prefs{Scale: 110}).ScaleFraction(); f != 1.1 {
		t.Errorf("ScaleFraction(110) = %v, want 1.1", f)
	}
	if f := (Prefs{}).ScaleFraction(); f != 1.0 {
		t.Errorf("ScaleFraction(default) = %v, want 1.0", f)
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
	got := Prefs{WeekStart: "x", DateStyle: "y", Theme: "z", Accent: "nope"}.Normalize()
	if got != Default() {
		t.Errorf("bad values should normalize to default, got %+v", got)
	}
	keep := Prefs{WeekStart: WeekMonday, DateStyle: DateLong, Theme: ThemeLight, Accent: "#abc", Compact: true, Scale: 110, ServerMode: ServerCloud, ServerURL: "http://127.0.0.1:8081", ServerToken: "dev-token", Motion: MotionSubtle}
	if keep.Normalize() != keep {
		t.Errorf("valid values should be preserved, got %+v", keep.Normalize())
	}
	if got := (Prefs{ServerMode: ServerMode("other")}).Normalize().ServerMode; got != ServerSelfHosted {
		t.Errorf("invalid server mode should default to self-hosted, got %q", got)
	}
}

func TestNormalizeThemeAndAccent(t *testing.T) {
	for _, th := range []Theme{ThemeDark, ThemeLight, ThemeSystem} {
		if got := (Prefs{Theme: th}).Normalize().Theme; got != th {
			t.Errorf("theme %q should be preserved, got %q", th, got)
		}
	}
	if got := (Prefs{}).Normalize().Theme; got != ThemeDark {
		t.Errorf("blank theme should default to dark, got %q", got)
	}
	if got := (Prefs{Accent: "#7c83ff"}).Normalize().Accent; got != "#7c83ff" {
		t.Errorf("valid accent should be preserved, got %q", got)
	}
	if got := (Prefs{Accent: "blue"}).Normalize().Accent; got != defaultAccent {
		t.Errorf("invalid accent should default, got %q", got)
	}
}

func TestIsHexColor(t *testing.T) {
	good := []string{"#fff", "#54b884", "#ABCDEF", "#000000"}
	bad := []string{"", "fff", "#ff", "#12345", "#gggggg", "54b884"}
	for _, s := range good {
		if !isHexColor(s) {
			t.Errorf("%q should be a valid hex color", s)
		}
	}
	for _, s := range bad {
		if isHexColor(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
