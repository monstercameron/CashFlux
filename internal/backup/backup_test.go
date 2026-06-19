package backup

import (
	"testing"
	"time"
)

func TestParseCadence(t *testing.T) {
	tests := []struct {
		in   string
		want Cadence
	}{
		{"weekly", Weekly},
		{"WEEKLY", Weekly},
		{" Monthly ", Monthly},
		{"off", Off},
		{"", Off},
		{"yearly", Off}, // unknown → Off, never nags
		{"garbage", Off},
	}
	for _, tc := range tests {
		if got := ParseCadence(tc.in); got != tc.want {
			t.Errorf("ParseCadence(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSchedules(t *testing.T) {
	if !Weekly.Schedules() || !Monthly.Schedules() {
		t.Error("Weekly/Monthly should schedule")
	}
	if Off.Schedules() {
		t.Error("Off should not schedule")
	}
}

func TestDue(t *testing.T) {
	last := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		cadence Cadence
		last    time.Time
		now     time.Time
		want    bool
	}{
		{"off never due", Off, last, last.AddDate(1, 0, 0), false},
		{"weekly not yet", Weekly, last, last.AddDate(0, 0, 6), false},
		{"weekly exactly due", Weekly, last, last.AddDate(0, 0, 7), true},
		{"weekly overdue", Weekly, last, last.AddDate(0, 0, 20), true},
		{"monthly not yet", Monthly, last, last.AddDate(0, 0, 20), false},
		{"monthly exactly due", Monthly, last, last.AddDate(0, 1, 0), true},
		{"monthly overdue", Monthly, last, last.AddDate(0, 2, 0), true},
		{"never backed up, weekly on", Weekly, time.Time{}, last, true},
		{"never backed up, off", Off, time.Time{}, last, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Due(tc.cadence, tc.last, tc.now); got != tc.want {
				t.Errorf("Due(%s) = %v, want %v", tc.cadence, got, tc.want)
			}
		})
	}
}

func TestNextDue(t *testing.T) {
	last := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	if _, ok := NextDue(Off, last); ok {
		t.Error("Off should not schedule a next due")
	}
	if n, ok := NextDue(Weekly, last); !ok || !n.Equal(last.AddDate(0, 0, 7)) {
		t.Errorf("weekly NextDue = %v ok=%v", n, ok)
	}
	if n, ok := NextDue(Monthly, last); !ok || !n.Equal(last.AddDate(0, 1, 0)) {
		t.Errorf("monthly NextDue = %v ok=%v", n, ok)
	}
}

func TestDaysSince(t *testing.T) {
	last := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		last time.Time
		now  time.Time
		want int
	}{
		{"zero is unknown", time.Time{}, last, 0},
		{"same day", last, last.Add(3 * time.Hour), 0},
		{"ten days", last, last.AddDate(0, 0, 10), 10},
		{"future clamps to zero", last, last.AddDate(0, 0, -5), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DaysSince(tc.last, tc.now); got != tc.want {
				t.Errorf("DaysSince = %d, want %d", got, tc.want)
			}
		})
	}
}
