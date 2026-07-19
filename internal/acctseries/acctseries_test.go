// SPDX-License-Identifier: MIT

package acctseries

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestAllDays(t *testing.T) {
	asOf := day(2026, time.July, 19)
	tests := []struct {
		name    string
		floor   int
		maxDays int
		dates   []time.Time
		want    int
	}{
		{"no dates -> floor", 90, 0, nil, 90},
		{"all zero -> floor", 90, 0, []time.Time{{}, {}}, 90},
		{"future date ignored -> floor", 90, 0, []time.Time{day(2026, time.August, 1)}, 90},
		{"same day ignored -> floor", 90, 0, []time.Time{asOf}, 90},
		{"span within floor stays floor", 90, 0, []time.Time{day(2026, time.June, 1)}, 90},
		{"one year back", 90, 0, []time.Time{day(2025, time.July, 20)}, 365},
		{"earliest of several wins", 90, 0, []time.Time{day(2026, time.January, 1), day(2024, time.July, 19), day(2026, time.June, 1)}, 731},
		{"cap applies", 90, 400, []time.Time{day(2010, time.July, 19)}, 400},
		{"cap not exceeded when under", 90, 400, []time.Time{day(2025, time.July, 20)}, 365},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := AllDays(asOf, tc.floor, tc.maxDays, tc.dates...); got != tc.want {
				t.Fatalf("AllDays = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestHasRange(t *testing.T) {
	if HasRange(90) {
		t.Fatal("90 days should not enable the range picker")
	}
	if !HasRange(91) {
		t.Fatal("more than 90 days should enable the range picker")
	}
	if HasRange(0) {
		t.Fatal("no history should not enable the range picker")
	}
}
