// SPDX-License-Identifier: MIT

package agemoney

import (
	"math"
	"testing"
)

func TestComputeLargeWeightedValuesStayExact(t *testing.T) {
	for _, test := range []struct {
		name  string
		flows []Flow
		want  int
		total int64
	}{
		{"product overflow", []Flow{{d(1), math.MaxInt64}, {d(3), -math.MaxInt64}}, 2, math.MaxInt64},
		{"rounding increment overflow", []Flow{{d(1), math.MaxInt64}, {d(2), -math.MaxInt64}}, 1, math.MaxInt64},
		{"sum across spends", []Flow{{d(1), math.MaxInt64}, {d(3), -math.MaxInt64 / 3}, {d(3), -math.MaxInt64 / 3}}, 2, 2 * (math.MaxInt64 / 3)},
		{"fractional age across lots", []Flow{{d(1), math.MaxInt64 / 2}, {d(2), math.MaxInt64 / 2}, {d(4), -(math.MaxInt64 - 1)}}, 3, math.MaxInt64 - 1},
		{"just below half rounds down", []Flow{{d(1), math.MaxInt64 / 2}, {d(2), math.MaxInt64/2 + 1}, {d(3), -math.MaxInt64}}, 1, math.MaxInt64},
		{"just above half rounds up", []Flow{{d(1), math.MaxInt64/2 + 1}, {d(2), math.MaxInt64 / 2}, {d(3), -math.MaxInt64}}, 2, math.MaxInt64},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := Compute(test.flows, Opts{})
			if !got.Ready || got.Overflow || got.Days != test.want || got.TotalAgedMinor != test.total {
				t.Fatalf("large weighted average = %+v, want %d days and %d matched", got, test.want, test.total)
			}
		})
	}
}

func TestComputeUnrepresentableWindowTotalIsUnavailable(t *testing.T) {
	flows := []Flow{{d(1), math.MaxInt64}, {d(1), 1}, {d(2), -math.MaxInt64}, {d(3), -1}}
	got := Compute(flows, Opts{})
	if got.Ready || !got.Overflow || got.Days != 0 || got.TotalAgedMinor != 0 || got.Capped || got.WindowCount != 2 || !got.WindowStart.Equal(d(2)) || !got.WindowEnd.Equal(d(3)) {
		t.Fatalf("unrepresentable exposed total became a valid or partial figure: %+v", got)
	}
	last := Compute(flows, Opts{Window: 1})
	if !last.Ready || last.Overflow || last.Days != 2 || last.TotalAgedMinor != 1 {
		t.Fatalf("old large spend poisoned a representable trailing window: %+v", last)
	}
}

func TestComputeMinInt64ExpenseConsumesFIFOHistory(t *testing.T) {
	flows := []Flow{{d(1), math.MaxInt64}, {d(1), 1}, {d(2), math.MinInt64}, {d(3), 100}, {d(4), -100}}
	got := Compute(flows, Opts{Window: 1})
	if !got.Ready || got.Overflow || got.Days != 1 || got.TotalAgedMinor != 100 {
		t.Fatalf("MinInt64 outflow did not consume earlier lots: %+v", got)
	}
	all := Compute(flows, Opts{})
	if all.Ready || !all.Overflow {
		t.Fatalf("MinInt64 matched total was narrowed into an available figure: %+v", all)
	}
}

func TestComputeLargeAmountsKeepConfiguredAgeCap(t *testing.T) {
	got := Compute([]Flow{{d(1), math.MaxInt64}, {d(500), -math.MaxInt64}}, Opts{MaxAgeDays: 30})
	if !got.Ready || got.Overflow || !got.Capped || got.Days != 30 || got.TotalAgedMinor != math.MaxInt64 {
		t.Fatalf("wide product changed the age cap: %+v", got)
	}
}

func BenchmarkComputeOrdinaryFIFO(b *testing.B) {
	flows := make([]Flow, 0, 2000)
	for i := range 1000 {
		flows = append(flows, Flow{d(i*15 + 1), 100000}, Flow{d(i*15 + 6), -100000})
	}
	b.ReportAllocs()
	for b.Loop() {
		Compute(flows, Opts{})
	}
}
