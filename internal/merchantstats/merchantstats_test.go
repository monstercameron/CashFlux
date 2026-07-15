// SPDX-License-Identifier: MIT

package merchantstats

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 12, 0, 0, 0, time.UTC)
}

func TestComputeTypicalAndDelta(t *testing.T) {
	now := d(2026, 6, 15)
	charges := []Charge{
		{d(2026, 4, 3), 3000},
		{d(2026, 5, 3), 3100},
		{d(2026, 6, 3), 3200}, // median of {3000,3100,3200} = 3100
	}
	s := Compute(charges, now, time.Sunday)
	if !s.Enough {
		t.Fatalf("3 charges should be enough")
	}
	if s.TypicalMinor != 3100 {
		t.Errorf("typical = %d, want 3100", s.TypicalMinor)
	}
	if got := s.DeltaVsTypical(3500); got != 400 {
		t.Errorf("delta = %d, want 400", got)
	}
	if len(s.Last12) != 3 || s.Last12[0] != 3000 || s.Last12[2] != 3200 {
		t.Errorf("last12 series wrong: %+v", s.Last12)
	}
}

func TestComputeNotEnough(t *testing.T) {
	now := d(2026, 6, 15)
	s := Compute([]Charge{{d(2026, 6, 1), 500}, {d(2026, 6, 8), 600}}, now, time.Sunday)
	if s.Enough {
		t.Errorf("2 charges should not be enough")
	}
	if s.Count != 2 {
		t.Errorf("count = %d, want 2", s.Count)
	}
}

func TestVisitsThisWeekAndMonth(t *testing.T) {
	now := d(2026, 6, 17) // a Wednesday; week (Sun start) began Jun 14
	charges := []Charge{
		{d(2026, 6, 2), 400},  // earlier this month, not this week
		{d(2026, 6, 15), 400}, // this week (Mon)
		{d(2026, 6, 16), 400}, // this week (Tue)
		{d(2026, 5, 20), 400}, // last month
	}
	s := Compute(charges, now, time.Sunday)
	if s.VisitsThisWeek != 2 {
		t.Errorf("visits this week = %d, want 2", s.VisitsThisWeek)
	}
	if s.VisitsThisMonth != 3 {
		t.Errorf("visits this month = %d, want 3", s.VisitsThisMonth)
	}
	if s.SpentThisMonth != 1200 {
		t.Errorf("spent this month = %d, want 1200", s.SpentThisMonth)
	}
}

func TestTypicalMonthExcludesCurrent(t *testing.T) {
	now := d(2026, 6, 15)
	charges := []Charge{
		{d(2026, 4, 3), 3000}, // Apr total 3000
		{d(2026, 5, 3), 5000}, // May total 5000
		{d(2026, 6, 3), 9999}, // current month excluded
	}
	s := Compute(charges, now, time.Sunday)
	// prior totals {3000, 5000} → median 4000
	if s.TypicalMonth != 4000 {
		t.Errorf("typical month = %d, want 4000", s.TypicalMonth)
	}
}

func TestLast12Caps(t *testing.T) {
	now := d(2026, 12, 31)
	var charges []Charge
	for i := 0; i < 15; i++ {
		charges = append(charges, Charge{d(2026, time.January, 1).AddDate(0, 0, i), int64(i)})
	}
	s := Compute(charges, now, time.Sunday)
	if len(s.Last12) != 12 {
		t.Fatalf("last12 len = %d, want 12", len(s.Last12))
	}
	// oldest of the last 12 is index 3 (value 3), newest 14.
	if s.Last12[0] != 3 || s.Last12[11] != 14 {
		t.Errorf("last12 window wrong: %+v", s.Last12)
	}
}
