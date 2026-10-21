// SPDX-License-Identifier: MIT

package reportview

import (
	"reflect"
	"testing"
	"time"
)

func TestInProgressMonthPreservesExactOpenStartClosedEnd(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	bounds := []time.Time{start, start.AddDate(0, 1, 0), start.AddDate(0, 2, 0)}
	before := append([]time.Time(nil), bounds...)
	for _, tc := range []struct {
		name string
		now  time.Time
		want int
	}{
		{"before first", start.Add(-time.Nanosecond), -1},
		{"at first start", start, -1},
		{"after first start", start.Add(time.Nanosecond), 0},
		{"before middle cutoff", bounds[1].Add(-time.Nanosecond), 0},
		{"at middle cutoff", bounds[1], 0},
		{"after middle cutoff", bounds[1].Add(time.Nanosecond), 1},
		{"at final cutoff", bounds[2], 1},
		{"after final cutoff", bounds[2].Add(time.Nanosecond), -1},
		{"same instant in another zone", bounds[1].In(time.FixedZone("west", -5*3600)), 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := InProgressMonth(bounds, tc.now); got != tc.want {
				t.Fatalf("month=%d want=%d", got, tc.want)
			}
		})
	}
	if !reflect.DeepEqual(bounds, before) {
		t.Fatal("month selection mutated bounds")
	}
	if InProgressMonth(nil, start) != -1 || InProgressMonth(bounds[:1], start) != -1 {
		t.Fatal("missing interval was treated as a month")
	}
}
