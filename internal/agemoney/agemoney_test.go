// SPDX-License-Identifier: MIT

package agemoney

import (
	"testing"
	"time"
)

// d builds a date at midnight UTC for the given day-of-January-2026 offset, so
// tests can express ages as simple day counts.
func d(dayOfMonth int) time.Time {
	return time.Date(2026, time.January, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func TestCompute(t *testing.T) {
	tests := []struct {
		name      string
		flows     []Flow
		opts      Opts
		wantReady bool
		wantDays  int
	}{
		{
			// Two income lots, one spend that straddles both.
			//   +$100 on Jan 1, +$100 on Jan 11, −$150 on Jan 21.
			// FIFO ages the first $100 at 20 days and the next $50 at 10 days:
			//   (100·20 + 50·10) / 150 = 2500/150 = 16.67 → 17.
			name: "two lots hand-computed",
			flows: []Flow{
				{Date: d(1), AmountMinor: 10000},
				{Date: d(11), AmountMinor: 10000},
				{Date: d(21), AmountMinor: -15000},
			},
			wantReady: true,
			wantDays:  17,
		},
		{
			// Empty ledger — nothing to age.
			name:      "empty",
			flows:     nil,
			wantReady: false,
		},
		{
			// Income only, no outflows yet — not ready.
			name: "all income",
			flows: []Flow{
				{Date: d(1), AmountMinor: 5000},
				{Date: d(2), AmountMinor: 5000},
			},
			wantReady: false,
		},
		{
			// Spending with no prior income to match against: the outflow is
			// unmatched, so the ledger's income history is too thin to trust.
			name: "not ready — spend precedes income",
			flows: []Flow{
				{Date: d(1), AmountMinor: -5000},
				{Date: d(2), AmountMinor: 10000},
			},
			wantReady: false,
		},
		{
			// Paycheck-to-paycheck: money is spent ~2 days after it lands, every
			// cycle. The buffer is thin, so the age is small.
			name: "paycheck to paycheck — small age",
			flows: []Flow{
				{Date: d(1), AmountMinor: 10000},
				{Date: d(3), AmountMinor: -10000},
				{Date: d(15), AmountMinor: 10000},
				{Date: d(17), AmountMinor: -10000},
				{Date: d(29), AmountMinor: 10000},
				{Date: d(31), AmountMinor: -10000},
			},
			wantReady: true,
			wantDays:  2,
		},
		{
			// A buffered household spends money it earned ~30 days earlier, every
			// cycle, so the age is large.
			name: "buffered — large age",
			flows: []Flow{
				{Date: d(1), AmountMinor: 10000},
				{Date: d(2), AmountMinor: 10000},
				{Date: d(3), AmountMinor: 10000},
				{Date: d(31), AmountMinor: -8000}, // ages against Jan 1 → 30 days
				{Date: d(32), AmountMinor: -8000}, // Jan 1 remainder + Jan 2 → ~30 days
				{Date: d(33), AmountMinor: -8000},
			},
			wantReady: true,
			wantDays:  30,
		},
		{
			// Transfers-excluded assumption: the caller drops transfer legs before
			// calling Compute, which here means they simply are not in the slice.
			// A zero-amount flow (a defensive stand-in) must not perturb the result:
			// this mirrors the two-lot case with an extra ignored flow and expects
			// the identical 17-day answer.
			name: "zero-amount flow ignored (transfers excluded upstream)",
			flows: []Flow{
				{Date: d(1), AmountMinor: 10000},
				{Date: d(5), AmountMinor: 0}, // e.g. a netted transfer the caller zeroed
				{Date: d(11), AmountMinor: 10000},
				{Date: d(21), AmountMinor: -15000},
			},
			wantReady: true,
			wantDays:  17,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.flows, tt.opts)
			if got.Ready != tt.wantReady {
				t.Fatalf("Ready = %v, want %v (result %+v)", got.Ready, tt.wantReady, got)
			}
			if tt.wantReady && got.Days != tt.wantDays {
				t.Errorf("Days = %d, want %d (result %+v)", got.Days, tt.wantDays, got)
			}
		})
	}
}

// TestWindowTrailing verifies only the trailing Window outflows count, so the
// figure tracks recent behavior and old spends drop out.
func TestWindowTrailing(t *testing.T) {
	flows := []Flow{
		{Date: d(1), AmountMinor: 100000},
		// One ancient, very-aged spend that would inflate a lifetime average.
		{Date: d(60), AmountMinor: -1000}, // 59 days old
		// Then two recent, fresh spends.
		{Date: d(61), AmountMinor: -1000}, // ages against Jan 1 → 60 days
	}
	// With Window=1 only the last outflow counts.
	got := Compute(flows, Opts{Window: 1})
	if !got.Ready {
		t.Fatalf("expected ready, got %+v", got)
	}
	if got.WindowCount != 1 {
		t.Errorf("WindowCount = %d, want 1", got.WindowCount)
	}
	if got.Days != 60 {
		t.Errorf("Days = %d, want 60 (only the last outflow)", got.Days)
	}
}

// TestBreakdown checks the explainability fields are populated for a ready result.
func TestBreakdown(t *testing.T) {
	flows := []Flow{
		{Date: d(1), AmountMinor: 10000},
		{Date: d(11), AmountMinor: 10000},
		{Date: d(21), AmountMinor: -15000},
	}
	got := Compute(flows, Opts{})
	if got.TotalAgedMinor != 15000 {
		t.Errorf("TotalAgedMinor = %d, want 15000", got.TotalAgedMinor)
	}
	if !got.WindowStart.Equal(d(21)) || !got.WindowEnd.Equal(d(21)) {
		t.Errorf("window = [%v, %v], want both Jan 21", got.WindowStart, got.WindowEnd)
	}
	if got.WindowCount != 1 {
		t.Errorf("WindowCount = %d, want 1", got.WindowCount)
	}
}

func TestComputeCapsAtMaxAge(t *testing.T) {
	// Income earned ~2 years before the spend → uncapped age ~730d; capped to 365.
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	flows := []Flow{
		{Date: base, AmountMinor: 100000},
		{Date: base.AddDate(2, 0, 0), AmountMinor: -50000},
	}
	got := Compute(flows, Opts{})
	if !got.Ready {
		t.Fatalf("expected Ready")
	}
	if !got.Capped {
		t.Fatalf("expected Capped=true, got Days=%d", got.Days)
	}
	if got.Days != DefaultMaxAgeDays {
		t.Errorf("Days = %d, want cap %d", got.Days, DefaultMaxAgeDays)
	}
}
