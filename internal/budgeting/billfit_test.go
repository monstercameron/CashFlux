// SPDX-License-Identifier: MIT

package budgeting

import "testing"

func TestFitBill(t *testing.T) {
	cases := []struct {
		name                    string
		limit, spent, bill      int64
		wantFits                bool
		wantOverBy, wantLeft    int64
	}{
		{"room to spare", 50000, 20000, 12000, true, 0, 18000},
		{"exactly to the limit fits", 50000, 38000, 12000, true, 0, 0},
		{"one over the limit", 50000, 38001, 12000, false, 1, 0},
		{"already over, bill makes it worse", 50000, 55000, 12000, false, 17000, 0},
		{"bill alone exceeds a fresh budget", 10000, 0, 12000, false, 2000, 0},
		{"fresh budget, bill fits", 120000, 0, 95000, true, 0, 25000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FitBill(c.limit, c.spent, c.bill)
			if got.Fits != c.wantFits || got.OverBy != c.wantOverBy || got.LeftAfter != c.wantLeft {
				t.Errorf("FitBill(%d,%d,%d) = %+v, want Fits=%v OverBy=%d LeftAfter=%d",
					c.limit, c.spent, c.bill, got, c.wantFits, c.wantOverBy, c.wantLeft)
			}
		})
	}
}
