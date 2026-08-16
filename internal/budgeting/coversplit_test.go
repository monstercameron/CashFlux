// SPDX-License-Identifier: MIT

package budgeting

import "testing"

func sumShares(m map[string]int64) int64 {
	var t int64
	for _, v := range m {
		t += v
	}
	return t
}

func TestCoverSplitEvenlyWithinCaps(t *testing.T) {
	shares, short := CoverSplit(10000, []CoverSplitInput{
		{ID: "a", Cap: 50000}, {ID: "b", Cap: 50000},
	})
	if short != 0 {
		t.Errorf("short = %d, want 0", short)
	}
	if shares["a"] != 5000 || shares["b"] != 5000 {
		t.Errorf("shares = %v, want 5000 each", shares)
	}
	if sumShares(shares) != 10000 {
		t.Errorf("shares sum to %d, want exactly 10000", sumShares(shares))
	}
}

// The defect: an uncapped weighted split assigned a source more than it had, and
// the this-period write then pushed it below its own spend with no warning.
func TestCoverSplitNeverExceedsACap(t *testing.T) {
	shares, short := CoverSplit(10000, []CoverSplitInput{
		{ID: "small", Cap: 1500}, {ID: "big", Cap: 50000},
	})
	if shares["small"] > 1500 {
		t.Errorf("small was asked for %d, more than its %d cap", shares["small"], 1500)
	}
	if sumShares(shares) != 10000 || short != 0 {
		t.Errorf("shares = %v short = %d; the shortfall should have rolled onto big", shares, short)
	}
	if shares["big"] != 8500 {
		t.Errorf("big = %d, want 8500 (the remainder after small was capped)", shares["big"])
	}
}

// When the selection genuinely cannot cover the amount, the split says so rather
// than silently moving less.
func TestCoverSplitReportsTheShortfall(t *testing.T) {
	shares, short := CoverSplit(10000, []CoverSplitInput{
		{ID: "a", Cap: 2000}, {ID: "b", Cap: 3000},
	})
	if sumShares(shares) != 5000 {
		t.Errorf("shares sum to %d, want 5000 — everything available", sumShares(shares))
	}
	if short != 5000 {
		t.Errorf("short = %d, want 5000", short)
	}
}

func TestCoverSplitRespectsWeights(t *testing.T) {
	shares, short := CoverSplit(9000, []CoverSplitInput{
		{ID: "a", Cap: 50000, Weight: 2}, {ID: "b", Cap: 50000, Weight: 1},
	})
	if short != 0 {
		t.Errorf("short = %d, want 0", short)
	}
	if shares["a"] != 6000 || shares["b"] != 3000 {
		t.Errorf("shares = %v, want a=6000 b=3000 (2:1)", shares)
	}
}

// A pinned source gives everything it has before the rest is split.
func TestCoverSplitPinnedGivesItsWholeCapFirst(t *testing.T) {
	shares, short := CoverSplit(10000, []CoverSplitInput{
		{ID: "pinned", Cap: 4000, Pinned: true}, {ID: "rest", Cap: 50000},
	})
	if shares["pinned"] != 4000 {
		t.Errorf("pinned = %d, want its whole 4000 cap", shares["pinned"])
	}
	if shares["rest"] != 6000 || short != 0 {
		t.Errorf("rest = %d short = %d, want 6000 and 0", shares["rest"], short)
	}
}

// A pinned source never gives more than the total asked for.
func TestCoverSplitPinnedIsCappedByTheTotal(t *testing.T) {
	shares, short := CoverSplit(3000, []CoverSplitInput{
		{ID: "pinned", Cap: 40000, Pinned: true}, {ID: "rest", Cap: 50000},
	})
	if shares["pinned"] != 3000 || shares["rest"] != 0 || short != 0 {
		t.Errorf("shares = %v short = %d, want the pinned source to give exactly 3000", shares, short)
	}
}

// Redistribution has to survive several capped sources in a row, not just one.
func TestCoverSplitRedistributesAcrossManyCaps(t *testing.T) {
	shares, short := CoverSplit(10000, []CoverSplitInput{
		{ID: "a", Cap: 500}, {ID: "b", Cap: 500}, {ID: "c", Cap: 500}, {ID: "d", Cap: 90000},
	})
	if short != 0 {
		t.Errorf("short = %d, want 0 — d could absorb the rest", short)
	}
	if sumShares(shares) != 10000 {
		t.Errorf("shares sum to %d, want 10000", sumShares(shares))
	}
	for _, id := range []string{"a", "b", "c"} {
		if shares[id] > 500 {
			t.Errorf("%s = %d, over its 500 cap", id, shares[id])
		}
	}
}

func TestCoverSplitEdgeCases(t *testing.T) {
	if shares, short := CoverSplit(0, []CoverSplitInput{{ID: "a", Cap: 100}}); shares != nil || short != 0 {
		t.Errorf("zero total = %v/%d, want nil/0", shares, short)
	}
	if shares, short := CoverSplit(500, nil); shares != nil || short != 500 {
		t.Errorf("no sources = %v/%d, want nil and the whole amount short", shares, short)
	}
	// A negative cap is not an amount; it must read as nothing to give.
	if shares, short := CoverSplit(500, []CoverSplitInput{{ID: "over", Cap: -900}}); shares["over"] != 0 || short != 500 {
		t.Errorf("negative cap = %v/%d, want 0 given and 500 short", shares, short)
	}
}
