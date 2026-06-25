// SPDX-License-Identifier: MIT

package ledger

import "testing"

func TestDelta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		curr     int64
		prev     int64
		wantKind DeltaKind
		wantPct  int64
	}{
		// Prior-zero cases.
		{
			name:     "prev=0 curr>0 → New",
			curr:     500,
			prev:     0,
			wantKind: DeltaNew,
		},
		{
			name:     "prev=0 curr<0 → New (negative value appeared)",
			curr:     -200,
			prev:     0,
			wantKind: DeltaNew,
		},
		{
			name:     "curr=0 prev>0 → Gone",
			curr:     0,
			prev:     1000,
			wantKind: DeltaGone,
		},
		{
			name:     "curr=0 prev<0 → Gone (negative value vanished)",
			curr:     0,
			prev:     -300,
			wantKind: DeltaGone,
		},
		{
			name:     "both=0 → Zero",
			curr:     0,
			prev:     0,
			wantKind: DeltaZero,
		},

		// Normal positive-baseline cases.
		{
			name:     "50% increase",
			curr:     150,
			prev:     100,
			wantKind: DeltaPctKind,
			wantPct:  50,
		},
		{
			name:     "25% decrease",
			curr:     75,
			prev:     100,
			wantKind: DeltaPctKind,
			wantPct:  -25,
		},
		{
			name:     "no change → 0%",
			curr:     200,
			prev:     200,
			wantKind: DeltaPctKind,
			wantPct:  0,
		},
		{
			name:     "100% increase (doubled)",
			curr:     200,
			prev:     100,
			wantKind: DeltaPctKind,
			wantPct:  100,
		},

		// Negative-baseline magnitude handling.
		// prev=-100 → |prev|=100; moving from -100 to -50 is +50% (improvement).
		{
			name:     "negative prev, less negative curr → positive pct",
			curr:     -50,
			prev:     -100,
			wantKind: DeltaPctKind,
			wantPct:  50,
		},
		// Moving from -100 to -200 worsens by 100%.
		{
			name:     "negative prev, more negative curr → negative pct",
			curr:     -200,
			prev:     -100,
			wantKind: DeltaPctKind,
			wantPct:  -100,
		},
		// Moving from -100 to 0 would be DeltaGone (handled above), but moving
		// from -100 to +50 is a 150% swing relative to |prev|=100.
		{
			name:     "negative prev, positive curr → large positive pct",
			curr:     50,
			prev:     -100,
			wantKind: DeltaPctKind,
			wantPct:  150,
		},

		// Integer truncation.
		{
			name:     "truncates toward zero (positive)",
			curr:     109,
			prev:     100,
			wantKind: DeltaPctKind,
			wantPct:  9, // 9.0 exactly
		},
		{
			name:     "truncates toward zero (fractional)",
			curr:     133,
			prev:     100,
			wantKind: DeltaPctKind,
			wantPct:  33, // 33.0 exactly
		},
		{
			name:     "truncates toward zero (remainder discarded)",
			curr:     101,
			prev:     300,
			wantKind: DeltaPctKind,
			wantPct:  -66, // -66.33… → -66
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Delta(tc.curr, tc.prev)
			if got.Kind != tc.wantKind {
				t.Errorf("Delta(%d, %d).Kind = %v, want %v", tc.curr, tc.prev, got.Kind, tc.wantKind)
			}
			if got.Kind == DeltaPctKind && got.Pct != tc.wantPct {
				t.Errorf("Delta(%d, %d).Pct = %d, want %d", tc.curr, tc.prev, got.Pct, tc.wantPct)
			}
		})
	}
}

func TestDeltaResultLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result DeltaResult
		want   string
	}{
		{
			name:   "New",
			result: DeltaResult{Kind: DeltaNew},
			want:   "New",
		},
		{
			name:   "Gone",
			result: DeltaResult{Kind: DeltaGone},
			want:   "Gone",
		},
		{
			name:   "Zero",
			result: DeltaResult{Kind: DeltaZero},
			want:   "—",
		},
		{
			name:   "positive pct",
			result: DeltaResult{Kind: DeltaPctKind, Pct: 42},
			want:   "+42%",
		},
		{
			name:   "negative pct",
			result: DeltaResult{Kind: DeltaPctKind, Pct: -17},
			want:   "-17%",
		},
		{
			name:   "zero pct (no change)",
			result: DeltaResult{Kind: DeltaPctKind, Pct: 0},
			want:   "+0%",
		},
		// Verify Label round-trips through Delta for common cases.
		{
			name:   "Delta 150→100 label",
			result: Delta(150, 100),
			want:   "+50%",
		},
		{
			name:   "Delta 0→500 label (New)",
			result: Delta(500, 0),
			want:   "New",
		},
		{
			name:   "Delta 500→0 label (Gone)",
			result: Delta(0, 500),
			want:   "Gone",
		},
		{
			name:   "Delta 0→0 label (Zero)",
			result: Delta(0, 0),
			want:   "—",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.result.Label()
			if got != tc.want {
				t.Errorf("Label() = %q, want %q (result=%+v)", got, tc.want, tc.result)
			}
		})
	}
}

func TestDeltaKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind DeltaKind
		want string
	}{
		{DeltaPctKind, "DeltaPctKind"},
		{DeltaNew, "DeltaNew"},
		{DeltaGone, "DeltaGone"},
		{DeltaZero, "DeltaZero"},
		{DeltaKind(99), "DeltaKind(99)"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.kind.String(); got != tc.want {
				t.Errorf("DeltaKind(%d).String() = %q, want %q", int(tc.kind), got, tc.want)
			}
		})
	}
}
