// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"testing"
	"time"
)

func TestOverFrequent(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	cases := []struct {
		name string
		ev   Evidence
		want bool
	}{
		{
			// A real monthly bill's count IS its span divided by its cadence.
			name: "monthly bill over two years",
			ev: Evidence{Count: 25, Cadence: CadenceMonthly,
				FirstSeen: day(2024, time.January, 9), LastSeen: day(2026, time.January, 9)},
			want: false,
		},
		{
			// Seven charges cannot be a yearly commitment inside three years.
			name: "seven yearly charges in three years",
			ev: Evidence{Count: 7, Cadence: CadenceAnnual,
				FirstSeen: day(2023, time.February, 2), LastSeen: day(2026, time.February, 2)},
			want: true,
		},
		{
			name: "genuine annual over five years",
			ev: Evidence{Count: 6, Cadence: CadenceAnnual,
				FirstSeen: day(2021, time.March, 1), LastSeen: day(2026, time.March, 1)},
			want: false,
		},
		{
			// A tolerance for the ordinary wobble of a real commitment: a monthly
			// bill that double-posted a couple of times must not be demoted.
			name: "monthly with a couple of double posts",
			ev: Evidence{Count: 14, Cadence: CadenceMonthly,
				FirstSeen: day(2025, time.January, 5), LastSeen: day(2026, time.January, 5)},
			want: false,
		},
		{
			name: "quarterly claim with monthly reality",
			ev: Evidence{Count: 12, Cadence: CadenceQuarterly,
				FirstSeen: day(2025, time.January, 5), LastSeen: day(2026, time.January, 5)},
			want: true,
		},
		{
			name: "too few to judge",
			ev: Evidence{Count: 3, Cadence: CadenceAnnual,
				FirstSeen: day(2025, time.January, 5), LastSeen: day(2026, time.January, 5)},
			want: false,
		},
		{
			name: "unknown cadence is never judged",
			ev: Evidence{Count: 40, Cadence: CadenceUnknown,
				FirstSeen: day(2025, time.January, 5), LastSeen: day(2026, time.January, 5)},
			want: false,
		},
		{
			name: "zero span is never judged",
			ev: Evidence{Count: 9, Cadence: CadenceMonthly,
				FirstSeen: day(2026, time.January, 5), LastSeen: day(2026, time.January, 5)},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := OverFrequent(tc.ev); got != tc.want {
				t.Errorf("OverFrequent(%+v) = %v, want %v", tc.ev, got, tc.want)
			}
		})
	}
}
