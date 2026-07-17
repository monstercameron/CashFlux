// SPDX-License-Identifier: MIT

package split

import "testing"

func TestByPercents(t *testing.T) {
	cases := []struct {
		name  string
		total int64
		bps   []int64
		want  []int64
	}{
		{"even halves", 10000, []int64{5000, 5000}, []int64{5000, 5000}},
		{"thirds sum exactly", 10000, []int64{3333, 3333, 3334}, []int64{3333, 3333, 3334}},
		{"remainder goes to largest fraction", 100, []int64{3333, 3333, 3334}, []int64{33, 33, 34}},
		{"uneven 70/30", 999, []int64{7000, 3000}, []int64{699, 300}},
		{"single line 100%", 4242, []int64{10000}, []int64{4242}},
		{"tiny total still sums", 1, []int64{5000, 5000}, []int64{1, 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ByPercents(tc.total, tc.bps)
			if err != nil {
				t.Fatalf("ByPercents: %v", err)
			}
			var sum int64
			for i, v := range got {
				sum += v
				if v != tc.want[i] {
					t.Errorf("line %d = %d, want %d (all: %v)", i, v, tc.want[i], got)
				}
			}
			if sum != tc.total {
				t.Errorf("shares sum to %d, want %d", sum, tc.total)
			}
		})
	}
}

func TestByPercentsErrors(t *testing.T) {
	cases := []struct {
		name string
		bps  []int64
	}{
		{"empty", nil},
		{"under 100%", []int64{5000, 4000}},
		{"over 100%", []int64{5000, 6000}},
		{"zero line", []int64{10000, 0}},
		{"negative line", []int64{11000, -1000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ByPercents(1000, tc.bps); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}
