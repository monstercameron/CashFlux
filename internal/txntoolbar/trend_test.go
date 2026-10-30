// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"reflect"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }

func TestRunningTotal(t *testing.T) {
	tests := []struct {
		name        string
		points      []DayPoint
		first, last time.Time
		want        []int64
	}{
		{"no points is a flat zero line", nil, day(1), day(3), []int64{0, 0, 0}},
		{"running sum by day", []DayPoint{{day(1), 500}, {day(3), -200}}, day(1), day(3), []int64{500, 500, 300}},
		{"same day adds up", []DayPoint{{day(2), 100}, {day(2), 50}}, day(1), day(2), []int64{0, 150}},
		{"points before the window set the start", []DayPoint{{day(1), 1000}, {day(3), 20}}, day(2), day(3), []int64{1000, 1020}},
		{"points after the window are ignored", []DayPoint{{day(5), 999}}, day(1), day(2), []int64{0, 0}},
		{"time of day does not change the day", []DayPoint{{day(2).Add(18 * time.Hour), 70}}, day(1), day(2), []int64{0, 70}},
		{"last before first is empty", []DayPoint{{day(1), 1}}, day(3), day(1), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RunningTotal(tt.points, tt.first, tt.last); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunningTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}
