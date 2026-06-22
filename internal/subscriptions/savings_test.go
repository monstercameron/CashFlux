package subscriptions

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// AnnualSavings
// ---------------------------------------------------------------------------

func TestAnnualSavings(t *testing.T) {
	base := d(2026, time.January, 1)
	subs := []Subscription{
		{Name: "Netflix", Cadence: CadenceMonthly, Amount: 1599, Last: base}, // annual = 1599*12 = 19188
		{Name: "Spotify", Cadence: CadenceMonthly, Amount: 999, Last: base},  // annual = 999*12  = 11988
		{Name: "Domain", Cadence: CadenceYearly, Amount: 1200, Last: base},   // annual = 1200
		{Name: "Gym", Cadence: CadenceWeekly, Amount: 2500, Last: base},      // annual = 2500*52 = 130000
	}

	tests := []struct {
		name     string
		selected map[string]bool
		want     int64
	}{
		{
			name:     "nothing selected",
			selected: map[string]bool{},
			want:     0,
		},
		{
			name:     "nil selected",
			selected: nil,
			want:     0,
		},
		{
			name:     "one monthly selected",
			selected: map[string]bool{"Netflix": true},
			want:     1599 * 12,
		},
		{
			name:     "one yearly selected",
			selected: map[string]bool{"Domain": true},
			want:     1200,
		},
		{
			name:     "one weekly selected",
			selected: map[string]bool{"Gym": true},
			want:     2500 * 52,
		},
		{
			name:     "two monthly selected",
			selected: map[string]bool{"Netflix": true, "Spotify": true},
			want:     1599*12 + 999*12,
		},
		{
			name:     "selected=false is ignored",
			selected: map[string]bool{"Netflix": true, "Spotify": false},
			want:     1599 * 12,
		},
		{
			name:     "unknown name is ignored",
			selected: map[string]bool{"Unknown": true},
			want:     0,
		},
		{
			name:     "all selected",
			selected: map[string]bool{"Netflix": true, "Spotify": true, "Domain": true, "Gym": true},
			want:     1599*12 + 999*12 + 1200 + 2500*52,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AnnualSavings(subs, tc.selected)
			if got != tc.want {
				t.Errorf("AnnualSavings = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAnnualSavings_EmptySubs(t *testing.T) {
	got := AnnualSavings(nil, map[string]bool{"Netflix": true})
	if got != 0 {
		t.Errorf("AnnualSavings(nil, ...) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// NeedsReview
// ---------------------------------------------------------------------------

func TestNeedsReview(t *testing.T) {
	now := time.Date(2026, time.June, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		sub     Subscription
		wantYes bool
	}{
		// Monthly — threshold is 62 days.
		{
			name:    "monthly recent (30 days ago) — no review",
			sub:     Subscription{Cadence: CadenceMonthly, Last: now.AddDate(0, 0, -30)},
			wantYes: false,
		},
		{
			name:    "monthly at threshold (62 days ago) — no review (not strictly over)",
			sub:     Subscription{Cadence: CadenceMonthly, Last: now.Add(-62 * 24 * time.Hour)},
			wantYes: false,
		},
		{
			name:    "monthly just over threshold (63 days ago) — needs review",
			sub:     Subscription{Cadence: CadenceMonthly, Last: now.AddDate(0, 0, -63)},
			wantYes: true,
		},
		{
			name:    "monthly very stale (120 days ago) — needs review",
			sub:     Subscription{Cadence: CadenceMonthly, Last: now.AddDate(0, 0, -120)},
			wantYes: true,
		},
		// Weekly — threshold is 14 days.
		{
			name:    "weekly recent (7 days ago) — no review",
			sub:     Subscription{Cadence: CadenceWeekly, Last: now.AddDate(0, 0, -7)},
			wantYes: false,
		},
		{
			name:    "weekly just over threshold (15 days ago) — needs review",
			sub:     Subscription{Cadence: CadenceWeekly, Last: now.AddDate(0, 0, -15)},
			wantYes: true,
		},
		// Yearly — threshold is 730 days.
		{
			name:    "yearly recent (365 days ago) — no review",
			sub:     Subscription{Cadence: CadenceYearly, Last: now.AddDate(0, 0, -365)},
			wantYes: false,
		},
		{
			name:    "yearly just over threshold (731 days ago) — needs review",
			sub:     Subscription{Cadence: CadenceYearly, Last: now.AddDate(0, 0, -731)},
			wantYes: true,
		},
		{
			name:    "yearly very stale (800 days ago) — needs review",
			sub:     Subscription{Cadence: CadenceYearly, Last: now.AddDate(0, 0, -800)},
			wantYes: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NeedsReview(tc.sub, now)
			if got != tc.wantYes {
				t.Errorf("NeedsReview = %v, want %v (Last=%s, Cadence=%s)",
					got, tc.wantYes,
					tc.sub.Last.Format("2006-01-02"), tc.sub.Cadence)
			}
		})
	}
}
