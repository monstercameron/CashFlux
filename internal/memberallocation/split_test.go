// SPDX-License-Identifier: MIT

package memberallocation

import "testing"

func TestSplitAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		owners []string
		want   []Share
	}{
		{"positive odd", 1001, []string{"a", "b"}, []Share{{"a", 501}, {"b", 500}}},
		{"negative odd", -1001, []string{"a", "b"}, []Share{{"a", -501}, {"b", -500}}},
		{"three owners", 10, []string{"a", "b", "c"}, []Share{{"a", 4}, {"b", 3}, {"c", 3}}},
		{"unassigned", 1001, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitAmount(tc.amount, tc.owners)
			if len(got) != len(tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("share %d = %#v, want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSplitAmountProperties(t *testing.T) {
	for amount := int64(-10000); amount <= 10000; amount++ {
		for count := 1; count <= 17; count++ {
			owners := make([]string, count)
			for i := range owners {
				owners[i] = string(rune('a' + i))
			}
			shares := SplitAmount(amount, owners)
			var sum, min, max int64
			for i, share := range shares {
				if share.MemberID != owners[i] {
					t.Fatalf("amount %d owners %d: order changed", amount, count)
				}
				if i == 0 || share.Amount < min {
					min = share.Amount
				}
				if i == 0 || share.Amount > max {
					max = share.Amount
				}
				sum += share.Amount
			}
			if sum != amount || max-min > 1 {
				t.Fatalf("amount %d owners %d: sum=%d spread=%d", amount, count, sum, max-min)
			}
		}
	}
}
