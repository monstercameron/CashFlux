// SPDX-License-Identifier: MIT

package staging

import (
	"testing"
)

func TestIMAAmountNormalizationAndOverflow(t *testing.T) {
	for _, tc := range []struct {
		text     string
		decimals int
		want     int64
		invalid  bool
	}{
		{"$45.00", 2, 4500, false}, {"1,234.56", 2, 123456, false}, {"(45.00)", 2, -4500, false},
		{"\u221245.00", 2, -4500, false}, {"45.00 DR", 2, -4500, false}, {"45.00 CR", 2, 4500, false},
		{"45.00-", 2, -4500, false}, {"\u20ac45.00", 2, 4500, false}, {"1500", 0, 1500, false},
		{"92233720368547758.07", 2, 9223372036854775807, false},
		{"-92233720368547758.07", 2, -9223372036854775807, false},
		{"92233720368547758.08", 2, 0, true}, {"--92233720368547758.08", 2, 0, true},
		{"($-92233720368547758.08)", 2, 0, true}, {"-45 DR", 2, 0, true},
		{"not money", 2, 0, true}, {"1.001", 2, 0, true},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, err := ParseAmount(tc.text, tc.decimals)
			if (err != nil) != tc.invalid || !tc.invalid && got != tc.want {
				t.Fatalf("ParseAmount(%q)=%d,%v want %d invalid=%v", tc.text, got, err, tc.want, tc.invalid)
			}
		})
	}
}
