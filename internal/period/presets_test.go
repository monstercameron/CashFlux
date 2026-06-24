// SPDX-License-Identifier: MIT

package period

import (
	"testing"
	"time"
)

func TestPrevious(t *testing.T) {
	// Given now = Jun 2026, Previous(Month) should be May 2026.
	now := d(2026, time.June, 15)
	w := Previous(Month, now, time.Monday)
	if w.Res != Month {
		t.Errorf("Previous resolution = %s, want month", w.Res)
	}
	if !w.From.Equal(d(2026, time.May, 1)) {
		t.Errorf("Previous(Month, Jun 15) From = %s, want 2026-05-01", w.From.Format("2006-01-02"))
	}
	if !w.IsSinglePeriod() {
		t.Error("Previous should be a single period")
	}
}

func TestYearToDate(t *testing.T) {
	now := d(2026, time.June, 15)
	w := YearToDate(now, time.Monday)
	if w.Res != Month {
		t.Errorf("YearToDate resolution = %s, want month", w.Res)
	}
	if !w.From.Equal(d(2026, time.January, 1)) {
		t.Errorf("YearToDate From = %s, want 2026-01-01", w.From.Format("2006-01-02"))
	}
	if !w.To.Equal(d(2026, time.June, 1)) {
		t.Errorf("YearToDate To = %s, want 2026-06-01", w.To.Format("2006-01-02"))
	}
}

func TestPriorYear(t *testing.T) {
	tests := []struct {
		name     string
		now      time.Time
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			name:     "mid-2026 yields 2025",
			now:      d(2026, time.June, 15),
			wantFrom: d(2025, time.January, 1),
			wantTo:   d(2025, time.January, 1),
		},
		{
			name:     "jan 2026 yields 2025",
			now:      d(2026, time.January, 1),
			wantFrom: d(2025, time.January, 1),
			wantTo:   d(2025, time.January, 1),
		},
		{
			name:     "dec 31 2025 yields 2024",
			now:      d(2025, time.December, 31),
			wantFrom: d(2024, time.January, 1),
			wantTo:   d(2024, time.January, 1),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := PriorYear(tc.now, time.Monday)
			if w.Res != Year {
				t.Errorf("PriorYear resolution = %s, want year", w.Res)
			}
			if !w.From.Equal(tc.wantFrom) {
				t.Errorf("PriorYear From = %s, want %s", w.From.Format("2006-01-02"), tc.wantFrom.Format("2006-01-02"))
			}
			if !w.To.Equal(tc.wantTo) {
				t.Errorf("PriorYear To = %s, want %s", w.To.Format("2006-01-02"), tc.wantTo.Format("2006-01-02"))
			}
			if !w.IsSinglePeriod() {
				t.Error("PriorYear should be a single period")
			}
			// The range should span the entire prior year.
			start, end := w.Range()
			if !start.Equal(tc.wantFrom) {
				t.Errorf("PriorYear Range start = %s, want %s", start.Format("2006-01-02"), tc.wantFrom.Format("2006-01-02"))
			}
			wantEnd := time.Date(tc.wantFrom.Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC)
			if !end.Equal(wantEnd) {
				t.Errorf("PriorYear Range end = %s, want %s", end.Format("2006-01-02"), wantEnd.Format("2006-01-02"))
			}
		})
	}
}
