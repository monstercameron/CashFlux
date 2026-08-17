// SPDX-License-Identifier: MIT

package whyover

import "testing"

func TestExplain(t *testing.T) {
	tests := []struct {
		name   string
		in     Input
		wantOK bool
		want   Reason
	}{
		{
			name:   "no limit is not a budget",
			in:     Input{LimitMinor: 0, SpentMinor: 50000},
			wantOK: false,
		},
		{
			name:   "within limit and pacing fine",
			in:     Input{LimitMinor: 50000, SpentMinor: 20000, ProjectedMinor: 40000, ElapsedBP: 5000},
			wantOK: false,
		},
		{
			name:   "pace projects over, past the noisy early window",
			in:     Input{LimitMinor: 50000, SpentMinor: 30000, ProjectedMinor: 60000, ElapsedBP: 5000, Count: 6},
			wantOK: true,
			want:   ReasonEarlyPace,
		},
		{
			name: "a wild projection on day two is not a finding",
			in: Input{LimitMinor: 50000, SpentMinor: 20000, ProjectedMinor: 300000,
				ElapsedBP: 700, Count: 1},
			wantOK: false,
		},
		{
			name: "one merchant accounts for most of the overage",
			in: Input{LimitMinor: 50000, SpentMinor: 68000, TopDriverMinor: 15000,
				Count: 9, PriorSpentMinor: 48000, PriorCount: 9},
			wantOK: true,
			want:   ReasonOneCharge,
		},
		{
			name: "more trips at the same prices",
			in: Input{LimitMinor: 50000, SpentMinor: 60000, TopDriverMinor: 4000,
				Count: 12, PriorSpentMinor: 40000, PriorCount: 8},
			wantOK: true,
			want:   ReasonMoreOften,
		},
		{
			name: "the same trips costing more",
			in: Input{LimitMinor: 50000, SpentMinor: 60000, TopDriverMinor: 4000,
				Count: 8, PriorSpentMinor: 40000, PriorCount: 8},
			wantOK: true,
			want:   ReasonPricier,
		},
		{
			// The regression this engine originally failed: the count went up 2.5×
			// while the average drifted 1.2× — past "steady" but short of "pricier".
			// A tight steady-band test dropped it to "no single cause"; it is a
			// count story and must be reported as one.
			name: "count jumped while the average merely drifted",
			in: Input{LimitMinor: 50000, SpentMinor: 90000, TopDriverMinor: 9000,
				Count: 20, PriorSpentMinor: 30000, PriorCount: 8},
			wantOK: true,
			want:   ReasonMoreOften,
		},
		{
			name: "both jumped: the bigger lever leads (price)",
			in: Input{LimitMinor: 50000, SpentMinor: 90000, TopDriverMinor: 9000,
				Count: 10, PriorSpentMinor: 20000, PriorCount: 5},
			wantOK: true,
			want:   ReasonPricier, // count 2.0x, average 2.25x
		},
		{
			name: "both jumped: the bigger lever leads (count)",
			in: Input{LimitMinor: 50000, SpentMinor: 90000, TopDriverMinor: 9000,
				Count: 24, PriorSpentMinor: 24000, PriorCount: 8},
			wantOK: true,
			want:   ReasonMoreOften, // count 3.0x, average 1.25x
		},
		{
			name: "spread out with no single cause",
			in: Input{LimitMinor: 50000, SpentMinor: 53000, TopDriverMinor: 1000,
				Count: 9, PriorSpentMinor: 49000, PriorCount: 9},
			wantOK: true,
			want:   ReasonSteady,
		},
		{
			name:   "over with no prior period to compare",
			in:     Input{LimitMinor: 50000, SpentMinor: 60000, TopDriverMinor: 2000, Count: 10},
			wantOK: true,
			want:   ReasonSteady,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Explain(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				return
			}
			if got.Reason != tc.want {
				t.Errorf("Reason = %v, want %v (result %+v)", got.Reason, tc.want, got)
			}
		})
	}
}

func TestExplain_Figures(t *testing.T) {
	got, ok := Explain(Input{
		LimitMinor: 50000, SpentMinor: 68000, TopDriverMinor: 15000,
		Count: 10, PriorSpentMinor: 45000, PriorCount: 9,
	})
	if !ok {
		t.Fatal("expected a finding")
	}
	if got.OverMinor != 18000 {
		t.Errorf("OverMinor = %d, want 18000", got.OverMinor)
	}
	if got.AvgMinor != 6800 {
		t.Errorf("AvgMinor = %d, want 6800", got.AvgMinor)
	}
	if got.PriorAvgMinor != 5000 {
		t.Errorf("PriorAvgMinor = %d, want 5000", got.PriorAvgMinor)
	}
	if got.CountDelta != 1 {
		t.Errorf("CountDelta = %d, want 1", got.CountDelta)
	}
	if !got.HasComparison {
		t.Error("HasComparison should be true when a prior period was supplied")
	}
}

// A merchant can outspend the overage several times over; the share is capped so
// the UI never states a percentage nobody can act on.
func TestExplain_TopDriverShareCapped(t *testing.T) {
	got, ok := Explain(Input{LimitMinor: 10000, SpentMinor: 40000, TopDriverMinor: 35000, Count: 3})
	if !ok {
		t.Fatal("expected a finding")
	}
	if got.TopDriverShareBP != bpScale {
		t.Errorf("TopDriverShareBP = %d, want %d", got.TopDriverShareBP, bpScale)
	}
	if got.Reason != ReasonOneCharge {
		t.Errorf("Reason = %v, want one-charge", got.Reason)
	}
}

func TestExplain_NoComparisonFlag(t *testing.T) {
	got, ok := Explain(Input{LimitMinor: 50000, SpentMinor: 60000, Count: 4})
	if !ok {
		t.Fatal("expected a finding")
	}
	if got.HasComparison {
		t.Error("HasComparison should be false with no prior period")
	}
	if got.PriorAvgMinor != 0 {
		t.Errorf("PriorAvgMinor = %d, want 0", got.PriorAvgMinor)
	}
}

func TestReasonString(t *testing.T) {
	tests := map[Reason]string{
		ReasonNone: "none", ReasonOneCharge: "one-charge", ReasonMoreOften: "more-often",
		ReasonPricier: "pricier", ReasonEarlyPace: "early-pace", ReasonSteady: "steady",
	}
	for r, want := range tests {
		if got := r.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", int(r), got, want)
		}
	}
}
