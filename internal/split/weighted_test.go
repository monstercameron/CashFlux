// SPDX-License-Identifier: MIT

package split

import "testing"

func TestByWeights(t *testing.T) {
	cases := []struct {
		name    string
		total   int64
		members []WeightedMember
		want    []int64
	}{
		{
			"2 to 1",
			3000,
			[]WeightedMember{{"a", 2}, {"b", 1}},
			[]int64{2000, 1000},
		},
		{
			"equal weights match Equal",
			1000,
			[]WeightedMember{{"a", 1}, {"b", 1}, {"c", 1}},
			[]int64{334, 333, 333}, // largest-remainder gives the extra cent to the first
		},
		{
			"income-proportional with remainder",
			10000,
			[]WeightedMember{{"a", 6000}, {"b", 3000}, {"c", 1000}}, // 60/30/10
			[]int64{6000, 3000, 1000},
		},
		{
			"remainder to largest fractional part",
			100,
			[]WeightedMember{{"a", 1}, {"b", 1}, {"c", 1}},
			[]int64{34, 33, 33},
		},
		{
			"zero-weight member gets nothing",
			900,
			[]WeightedMember{{"a", 2}, {"b", 1}, {"c", 0}},
			[]int64{600, 300, 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ByWeights(tc.total, tc.members)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d shares, want %d: %+v", len(got), len(tc.want), got)
			}
			var sum int64
			for i, s := range got {
				if s.MemberID != tc.members[i].MemberID {
					t.Errorf("share %d id = %q, want %q", i, s.MemberID, tc.members[i].MemberID)
				}
				if s.Amount != tc.want[i] {
					t.Errorf("share %d = %d, want %d", i, s.Amount, tc.want[i])
				}
				sum += s.Amount
			}
			if sum != tc.total {
				t.Errorf("shares sum to %d, want total %d", sum, tc.total)
			}
		})
	}
}

func TestByWeightsNoBasis(t *testing.T) {
	if got := ByWeights(1000, nil); got != nil {
		t.Errorf("no members should return nil, got %+v", got)
	}
	if got := ByWeights(1000, []WeightedMember{{"a", 0}, {"b", -5}}); got != nil {
		t.Errorf("all non-positive weights should return nil, got %+v", got)
	}
}

func TestByWeightsExactSumLargeRemainder(t *testing.T) {
	// 100 split 1:1:1:1:1:1:1 (7 ways) — leftover distributed so it still sums.
	members := []WeightedMember{{"a", 1}, {"b", 1}, {"c", 1}, {"d", 1}, {"e", 1}, {"f", 1}, {"g", 1}}
	got := ByWeights(100, members)
	var sum int64
	for _, s := range got {
		sum += s.Amount
	}
	if sum != 100 {
		t.Errorf("sum = %d, want 100", sum)
	}
}
