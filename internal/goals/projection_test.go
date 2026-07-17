// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"
)

func TestProjectWithGrowth(t *testing.T) {
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)

	t.Run("already at target is zero months", func(t *testing.T) {
		p := ProjectWithGrowth(100000, 100000, 5000, 700, from)
		if !p.Reachable || p.Months != 0 {
			t.Fatalf("got %+v, want reachable in 0 months", p)
		}
	})

	t.Run("no growth: pure contribution pace", func(t *testing.T) {
		// $0 -> $1200 target at $100/mo, 0% return => exactly 12 months.
		p := ProjectWithGrowth(0, 120000, 10000, 0, from)
		if !p.Reachable || p.Months != 12 {
			t.Fatalf("got %+v, want 12 months", p)
		}
		if !p.Date.Equal(from.AddDate(0, 12, 0)) {
			t.Errorf("date = %v, want %v", p.Date, from.AddDate(0, 12, 0))
		}
	})

	t.Run("growth reaches the target sooner than no-growth", func(t *testing.T) {
		// A long-horizon goal where compounding matters: $10k now, $500k target,
		// $1000/mo, 7% APR. It must be reachable, and reach sooner than 0% would.
		p := ProjectWithGrowth(1_000_000, 50_000_000, 100_000, 700, from)
		if !p.Reachable {
			t.Fatalf("expected reachable, got %+v", p)
		}
		if p.MonthsNoGrowth <= p.Months {
			t.Errorf("growth (%d mo) should beat no-growth (%d mo)", p.Months, p.MonthsNoGrowth)
		}
	})

	t.Run("no contribution and no growth under target is unreachable", func(t *testing.T) {
		p := ProjectWithGrowth(1000, 100000, 0, 0, from)
		if p.Reachable {
			t.Errorf("expected unreachable, got %+v", p)
		}
	})

	t.Run("growth alone (no contribution) can still reach", func(t *testing.T) {
		// $100k at 10% APR with no contribution reaches $110k within ~12 months.
		p := ProjectWithGrowth(10_000_000, 11_000_000, 0, 1000, from)
		if !p.Reachable {
			t.Fatalf("growth alone should reach, got %+v", p)
		}
		if p.Months <= 0 || p.Months > 14 {
			t.Errorf("months = %d, want ~12", p.Months)
		}
	})
}
