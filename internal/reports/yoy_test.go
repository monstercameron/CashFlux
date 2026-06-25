// SPDX-License-Identifier: MIT

package reports

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
)

// d builds a UTC midnight date for use in window tests (distinct from dt, which
// builds noon times for transaction tests).
func yd(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestYoYPrior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    period.Window
		wantFrom time.Time
		wantTo   time.Time
	}{
		{
			// A single monthly window for June 2026 → June 2025.
			name: "monthly window shifts back one year",
			input: period.Window{
				Res:       period.Month,
				From:      yd(2026, time.June, 1),
				To:        yd(2026, time.June, 1),
				WeekStart: time.Sunday,
			},
			wantFrom: yd(2025, time.June, 1),
			wantTo:   yd(2025, time.June, 1),
		},
		{
			// A multi-month range Apr–Jun 2026 → Apr–Jun 2025.
			name: "multi-month range both bounds shift back one year",
			input: period.Window{
				Res:       period.Month,
				From:      yd(2026, time.April, 1),
				To:        yd(2026, time.June, 1),
				WeekStart: time.Monday,
			},
			wantFrom: yd(2025, time.April, 1),
			wantTo:   yd(2025, time.June, 1),
		},
		{
			// Weekly window: week starting Mon 2026-01-05 → Mon 2025-01-06
			// (same calendar arithmetic — AddDate(-1,0,0) on the day, not the ISO week).
			name: "weekly window shifts back one year",
			input: period.Window{
				Res:       period.Week,
				From:      yd(2026, time.January, 5),
				To:        yd(2026, time.January, 5),
				WeekStart: time.Monday,
			},
			wantFrom: yd(2025, time.January, 5),
			wantTo:   yd(2025, time.January, 5),
		},
		{
			// Q1 2026 window (Jan 1 → Mar 1 anchors) → Q1 2025.
			name: "quarterly window shifts back one year",
			input: period.Window{
				Res:       period.Quarter,
				From:      yd(2026, time.January, 1),
				To:        yd(2026, time.March, 1),
				WeekStart: time.Sunday,
			},
			wantFrom: yd(2025, time.January, 1),
			wantTo:   yd(2025, time.March, 1),
		},
		{
			// Leap-day boundary: Feb 29 2024 (leap year) shifted back yields
			// Mar 1 2023 because 2023 has no Feb 29. This is Go's documented
			// AddDate normalisation (Feb 29 + (-1 year) = Mar 1 non-leap).
			name: "leap day Feb 29 normalises to Mar 1 in non-leap year",
			input: period.Window{
				Res:       period.Month,
				From:      yd(2024, time.February, 29),
				To:        yd(2024, time.February, 29),
				WeekStart: time.Sunday,
			},
			wantFrom: yd(2023, time.March, 1),
			wantTo:   yd(2023, time.March, 1),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := YoYPrior(tc.input)

			// Resolution and WeekStart must be unchanged.
			if got.Res != tc.input.Res {
				t.Errorf("Res = %q, want %q", got.Res, tc.input.Res)
			}
			if got.WeekStart != tc.input.WeekStart {
				t.Errorf("WeekStart = %v, want %v", got.WeekStart, tc.input.WeekStart)
			}

			if !got.From.Equal(tc.wantFrom) {
				t.Errorf("From = %s, want %s",
					got.From.Format("2006-01-02"), tc.wantFrom.Format("2006-01-02"))
			}
			if !got.To.Equal(tc.wantTo) {
				t.Errorf("To = %s, want %s",
					got.To.Format("2006-01-02"), tc.wantTo.Format("2006-01-02"))
			}
		})
	}
}

// TestYoYPriorIsNotShiftMinus1 confirms that YoYPrior is semantically distinct
// from Window.Shift(-1): for a monthly window, Shift(-1) moves back one month
// while YoYPrior moves back one year.
func TestYoYPriorIsNotShiftMinus1(t *testing.T) {
	t.Parallel()

	w := period.Window{
		Res:       period.Month,
		From:      yd(2026, time.June, 1),
		To:        yd(2026, time.June, 1),
		WeekStart: time.Sunday,
	}
	shiftedBack := w.Shift(-1) // May 2026
	yoy := YoYPrior(w)         // June 2025

	if shiftedBack.From.Equal(yoy.From) {
		t.Errorf("YoYPrior and Shift(-1) must not produce the same From anchor (%s)", yoy.From.Format("2006-01-02"))
	}
	wantShift := yd(2026, time.May, 1)
	if !shiftedBack.From.Equal(wantShift) {
		t.Errorf("Shift(-1).From = %s, want %s", shiftedBack.From.Format("2006-01-02"), wantShift.Format("2006-01-02"))
	}
	wantYoY := yd(2025, time.June, 1)
	if !yoy.From.Equal(wantYoY) {
		t.Errorf("YoYPrior.From = %s, want %s", yoy.From.Format("2006-01-02"), wantYoY.Format("2006-01-02"))
	}
}
