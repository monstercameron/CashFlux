// SPDX-License-Identifier: MIT

package period

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestValid(t *testing.T) {
	for _, r := range []Resolution{Week, Month, Quarter, Year} {
		if !r.Valid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if Resolution("decade").Valid() {
		t.Error("decade should be invalid")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		res  Resolution
		in   time.Time
		want time.Time
	}{
		{Month, d(2026, time.June, 15), d(2026, time.June, 1)},
		{Quarter, d(2026, time.June, 15), d(2026, time.April, 1)},       // Q2
		{Quarter, d(2026, time.January, 9), d(2026, time.January, 1)},   // Q1
		{Quarter, d(2026, time.December, 31), d(2026, time.October, 1)}, // Q4
		{Year, d(2026, time.June, 15), d(2026, time.January, 1)},
		{Year, d(2026, time.December, 31), d(2026, time.January, 1)},
		{Week, d(2026, time.June, 17), d(2026, time.June, 15)}, // Wed -> Mon
	}
	for _, tt := range tests {
		if got := Truncate(tt.res, tt.in, time.Monday); !got.Equal(tt.want) {
			t.Errorf("Truncate(%s, %s) = %s, want %s", tt.res, tt.in.Format("2006-01-02"), got.Format("2006-01-02"), tt.want.Format("2006-01-02"))
		}
	}
}

func TestStep(t *testing.T) {
	tests := []struct {
		res   Resolution
		in    time.Time
		delta int
		want  time.Time
	}{
		{Month, d(2026, time.June, 1), 2, d(2026, time.August, 1)},
		{Month, d(2026, time.January, 1), -1, d(2025, time.December, 1)},
		{Quarter, d(2026, time.April, 1), 1, d(2026, time.July, 1)},
		{Year, d(2026, time.January, 1), 1, d(2027, time.January, 1)},
		{Year, d(2026, time.January, 1), -1, d(2025, time.January, 1)},
		{Quarter, d(2026, time.January, 1), -1, d(2025, time.October, 1)},
		{Week, d(2026, time.June, 15), 1, d(2026, time.June, 22)},
		{Week, d(2026, time.June, 15), -2, d(2026, time.June, 1)},
	}
	for _, tt := range tests {
		if got := Step(tt.res, tt.in, tt.delta); !got.Equal(tt.want) {
			t.Errorf("Step(%s, %s, %d) = %s, want %s", tt.res, tt.in.Format("2006-01-02"), tt.delta, got.Format("2006-01-02"), tt.want.Format("2006-01-02"))
		}
	}
}

func TestLabel(t *testing.T) {
	tests := []struct {
		res  Resolution
		in   time.Time
		want string
	}{
		{Month, d(2026, time.June, 15), "Jun 2026"},
		{Quarter, d(2026, time.June, 15), "Q2 2026"},
		{Quarter, d(2026, time.November, 3), "Q4 2026"},
		{Week, d(2026, time.June, 17), "Jun 15 – Jun 21"}, // Mon..Sun
	}
	for _, tt := range tests {
		if got := Label(tt.res, tt.in, time.Monday); got != tt.want {
			t.Errorf("Label(%s, %s) = %q, want %q", tt.res, tt.in.Format("2006-01-02"), got, tt.want)
		}
	}
}

func TestRange(t *testing.T) {
	// June through August 2026 at month resolution: [Jun 1, Sep 1).
	start, end := Range(Month, d(2026, time.June, 10), d(2026, time.August, 20), time.Monday)
	if !start.Equal(d(2026, time.June, 1)) || !end.Equal(d(2026, time.September, 1)) {
		t.Errorf("Range month = [%s, %s), want [2026-06-01, 2026-09-01)", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	// Quarter: Q2 through Q3 -> [Apr 1, Oct 1).
	start, end = Range(Quarter, d(2026, time.May, 1), d(2026, time.September, 1), time.Monday)
	if !start.Equal(d(2026, time.April, 1)) || !end.Equal(d(2026, time.October, 1)) {
		t.Errorf("Range quarter = [%s, %s), want [2026-04-01, 2026-10-01)", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	// to before from clamps to a single unit.
	start, end = Range(Month, d(2026, time.June, 1), d(2026, time.March, 1), time.Monday)
	if !start.Equal(d(2026, time.June, 1)) || !end.Equal(d(2026, time.July, 1)) {
		t.Errorf("Range clamp = [%s, %s), want [2026-06-01, 2026-07-01)", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}
