package savings

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// RoundUpDelta
// ---------------------------------------------------------------------------

func TestRoundUpDelta(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		amount      int64
		granularity int64
		want        int64
	}{
		// mid-value: 347 rounded to next 100 → 400, delta = 53
		{name: "mid_value_100", amount: 347, granularity: 100, want: 53},
		// on boundary: already at 500, delta = 0
		{name: "on_boundary_100", amount: 500, granularity: 100, want: 0},
		// $5 granularity (500 minor units): 1234 → 1500, delta = 266
		{name: "five_dollar_granularity", amount: 1234, granularity: 500, want: 266},
		// $5 granularity on boundary
		{name: "five_dollar_on_boundary", amount: 1500, granularity: 500, want: 0},
		// granularity = 0 → always 0
		{name: "zero_granularity", amount: 347, granularity: 0, want: 0},
		// granularity negative → always 0
		{name: "negative_granularity", amount: 347, granularity: -10, want: 0},
		// amount = 0 on any boundary
		{name: "zero_amount", amount: 0, granularity: 100, want: 0},
		// granularity = 1 → always on boundary
		{name: "unit_granularity", amount: 999, granularity: 1, want: 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RoundUpDelta(tc.amount, tc.granularity)
			if got != tc.want {
				t.Errorf("RoundUpDelta(%d, %d) = %d; want %d",
					tc.amount, tc.granularity, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SurplusMinor
// ---------------------------------------------------------------------------

func TestSurplusMinor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		liquid       int64
		billsDue     int64
		goalContribs int64
		cap          int64
		want         int64
	}{
		// positive surplus, no cap
		{name: "positive_no_cap", liquid: 100000, billsDue: 40000, goalContribs: 10000, cap: 0, want: 50000},
		// negative surplus → 0
		{name: "negative_surplus", liquid: 5000, billsDue: 40000, goalContribs: 0, cap: 0, want: 0},
		// surplus exactly 0
		{name: "zero_surplus", liquid: 50000, billsDue: 30000, goalContribs: 20000, cap: 0, want: 0},
		// capped: surplus 50000 but cap 20000
		{name: "capped", liquid: 100000, billsDue: 40000, goalContribs: 10000, cap: 20000, want: 20000},
		// cap equals surplus (boundary)
		{name: "cap_equals_surplus", liquid: 100000, billsDue: 40000, goalContribs: 10000, cap: 50000, want: 50000},
		// cap > surplus: cap does not inflate
		{name: "cap_larger_than_surplus", liquid: 100000, billsDue: 40000, goalContribs: 10000, cap: 999999, want: 50000},
		// cap ≤ 0 treated as no cap
		{name: "cap_zero_uncapped", liquid: 100000, billsDue: 40000, goalContribs: 10000, cap: 0, want: 50000},
		{name: "cap_negative_uncapped", liquid: 100000, billsDue: 40000, goalContribs: 10000, cap: -1, want: 50000},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SurplusMinor(tc.liquid, tc.billsDue, tc.goalContribs, tc.cap)
			if got != tc.want {
				t.Errorf("SurplusMinor(%d, %d, %d, %d) = %d; want %d",
					tc.liquid, tc.billsDue, tc.goalContribs, tc.cap, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsScheduleDue
// ---------------------------------------------------------------------------

func mustParse(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestIsScheduleDue(t *testing.T) {
	t.Parallel()
	now := mustParse("2026-06-25")

	cases := []struct {
		name    string
		lastRun time.Time
		cadence string
		now     time.Time
		want    bool
	}{
		// zero lastRun → always due
		{name: "zero_lastRun_daily", lastRun: time.Time{}, cadence: "daily", now: now, want: true},
		{name: "zero_lastRun_monthly", lastRun: time.Time{}, cadence: "monthly", now: now, want: true},

		// daily: last run today → not due
		{name: "daily_same_day", lastRun: now, cadence: "daily", now: now, want: false},
		// daily: last run exactly 1 day ago → due
		{name: "daily_exactly_1d", lastRun: mustParse("2026-06-24"), cadence: "daily", now: now, want: true},
		// daily: last run 23 hours ago (sub-day boundary) → not due
		{name: "daily_23h_ago", lastRun: now.Add(-23 * time.Hour), cadence: "daily", now: now, want: false},

		// weekly: 6 days ago → not due
		{name: "weekly_6d", lastRun: mustParse("2026-06-19"), cadence: "weekly", now: now, want: false},
		// weekly: exactly 7 days ago → due
		{name: "weekly_7d", lastRun: mustParse("2026-06-18"), cadence: "weekly", now: now, want: true},

		// biweekly: 13 days ago → not due
		{name: "biweekly_13d", lastRun: mustParse("2026-06-12"), cadence: "biweekly", now: now, want: false},
		// biweekly: exactly 14 days ago → due
		{name: "biweekly_14d", lastRun: mustParse("2026-06-11"), cadence: "biweekly", now: now, want: true},

		// monthly: 29 days ago in a 30-day month → not due
		{name: "monthly_29d", lastRun: mustParse("2026-05-27"), cadence: "monthly", now: now, want: false},
		// monthly: exactly 1 month ago → due
		{name: "monthly_1mo", lastRun: mustParse("2026-05-25"), cadence: "monthly", now: now, want: true},

		// unknown cadence → false
		{name: "unknown_cadence", lastRun: mustParse("2020-01-01"), cadence: "yearly", now: now, want: false},
		{name: "empty_cadence", lastRun: mustParse("2020-01-01"), cadence: "", now: now, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsScheduleDue(tc.lastRun, tc.cadence, tc.now)
			if got != tc.want {
				t.Errorf("IsScheduleDue(%v, %q, %v) = %v; want %v",
					tc.lastRun, tc.cadence, tc.now, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PeriodKey
// ---------------------------------------------------------------------------

func TestPeriodKey(t *testing.T) {
	t.Parallel()

	// Reference date: Wednesday 2026-06-24 (ISO week 26 of 2026).
	ref := time.Date(2026, 6, 24, 15, 30, 0, 0, time.UTC)

	cases := []struct {
		name   string
		t      time.Time
		period string
		want   string
	}{
		// monthly
		{name: "monthly_june", t: ref, period: "monthly", want: "2026-06"},
		{name: "monthly_jan", t: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), period: "monthly", want: "2026-01"},

		// weekly (ISO 8601)
		{name: "weekly_w26", t: ref, period: "weekly", want: "2026-W26"},
		// ISO week 1 of 2026: Jan 5 2026 is Monday of W02 — use Jan 1 which is W01
		{name: "weekly_w01", t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), period: "weekly", want: "2026-W01"},

		// daily
		{name: "daily_ref", t: ref, period: "daily", want: "2026-06-24"},
		{name: "daily_epoch", t: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), period: "daily", want: "1970-01-01"},

		// biweekly — two dates in the same 14-day bucket must produce the same key.
		// Jun 8 and Jun 14 are both in epoch-anchored bucket 1472.
		{name: "biweekly_same_bucket_a", t: time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), period: "biweekly",
			want: PeriodKey(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), "biweekly")},
		{name: "biweekly_same_bucket_b", t: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), period: "biweekly",
			want: PeriodKey(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), "biweekly")},

		// default (unknown period) → monthly format
		{name: "default_unknown", t: ref, period: "quarterly", want: "2026-06"},
		{name: "default_empty", t: ref, period: "", want: "2026-06"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := PeriodKey(tc.t, tc.period)
			if got != tc.want {
				t.Errorf("PeriodKey(%v, %q) = %q; want %q", tc.t, tc.period, got, tc.want)
			}
		})
	}

	// Additional invariant: two dates in different 14-day buckets must NOT share a key.
	// Jun 1 (bucket 1471) vs Jun 8 (bucket 1472).
	t.Run("biweekly_different_buckets", func(t *testing.T) {
		t.Parallel()
		a := PeriodKey(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), "biweekly")
		b := PeriodKey(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC), "biweekly")
		if a == b {
			t.Errorf("expected different bucket keys; both = %q", a)
		}
	})
}
