// SPDX-License-Identifier: MIT

package goals

import "testing"

func sum(xs []int64) int64 {
	var s int64
	for _, x := range xs {
		s += x
	}
	return s
}

func TestSplitEvenBasic(t *testing.T) {
	// $900 across 3 roomy accounts → $300 each.
	got := SplitEarmark(90000, []int64{100000, 100000, 100000}, SplitEven)
	for i, v := range got {
		if v != 30000 {
			t.Fatalf("even[%d] = %d, want 30000 (%v)", i, v, got)
		}
	}
}

func TestSplitEvenWaterfallsCappedAccounts(t *testing.T) {
	// $900 across accounts with headroom [100, 1000, 1000] (cents scaled): the tiny account
	// caps at 100 and the overflow spreads to the other two → 100 / 400 / 400.
	got := SplitEarmark(900, []int64{100, 1000, 1000}, SplitEven)
	if got[0] != 100 || got[1] != 400 || got[2] != 400 {
		t.Fatalf("waterfall = %v, want [100 400 400]", got)
	}
	if sum(got) != 900 {
		t.Fatalf("sum = %d, want 900", sum(got))
	}
}

func TestSplitProportionalByHeadroom(t *testing.T) {
	// $1000 split proportionally over headroom [1000, 3000] → 1:3 → 250 / 750.
	got := SplitEarmark(1000, []int64{1000, 3000}, SplitProportional)
	if got[0] != 250 || got[1] != 750 {
		t.Fatalf("proportional = %v, want [250 750]", got)
	}
	if sum(got) != 1000 {
		t.Fatalf("sum = %d, want 1000", sum(got))
	}
}

func TestSplitProportionalRemainderExactSum(t *testing.T) {
	// A total that doesn't divide evenly must still sum EXACTLY to total (largest remainder):
	// 100 over three roomy equal accounts → 34/33/33 (the extra cent lands on one account).
	got := SplitEarmark(100, []int64{1000, 1000, 1000}, SplitProportional)
	if sum(got) != 100 {
		t.Fatalf("sum = %d, want 100 (%v)", sum(got), got)
	}
	if got[0] != 34 || got[1] != 33 || got[2] != 33 {
		t.Fatalf("largest-remainder = %v, want [34 33 33]", got)
	}
	// When the total exceeds capacity, it clamps to capacity.
	if s := sum(SplitEarmark(100, []int64{1, 1, 1}, SplitProportional)); s != 3 {
		t.Fatalf("clamped sum = %d, want 3 (capacity)", s)
	}
}

func TestSplitClampsToCapacityAndRespectsCaps(t *testing.T) {
	for _, mode := range []SplitMode{SplitEven, SplitProportional} {
		avail := []int64{500, 200, 0, 300}     // capacity 1000; one account has no room
		got := SplitEarmark(5000, avail, mode) // ask for more than exists
		if sum(got) != 1000 {
			t.Fatalf("%s clamp sum = %d, want 1000", mode, sum(got))
		}
		for i := range avail {
			if got[i] < 0 || got[i] > maxI64(avail[i], 0) {
				t.Fatalf("%s out[%d]=%d exceeds cap %d", mode, i, got[i], avail[i])
			}
		}
		if got[2] != 0 {
			t.Fatalf("%s gave %d to a zero-headroom account", mode, got[2])
		}
	}
}

func TestSplitEdgeCases(t *testing.T) {
	if got := SplitEarmark(0, []int64{100}, SplitEven); got[0] != 0 {
		t.Fatalf("zero total should give nothing, got %v", got)
	}
	if got := SplitEarmark(100, nil, SplitEven); len(got) != 0 {
		t.Fatalf("no accounts should give empty, got %v", got)
	}
	if got := SplitEarmark(100, []int64{-50, 0}, SplitProportional); sum(got) != 0 {
		t.Fatalf("no positive headroom should give nothing, got %v", got)
	}
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
