// SPDX-License-Identifier: MIT

package inflation

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestRealMinorDeflates(t *testing.T) {
	// $100,000 in 20 years at 3% is worth about $55,368 today.
	got, ok := RealMinor(10000000, 3, 20)
	if !ok {
		t.Fatal("not ok")
	}
	if !near(float64(got), 5536758, 200) {
		t.Errorf("RealMinor = %d, want ~5536758", got)
	}
	// Further out is worth less.
	further, _ := RealMinor(10000000, 3, 30)
	if further >= got {
		t.Errorf("30 years (%d) is not worth less than 20 (%d)", further, got)
	}
}

// A zero rate is the IDENTITY, not an error: plenty of callers have no
// assumption configured, and refusing would force every one of them to branch.
func TestZeroRateIsTheIdentity(t *testing.T) {
	got, ok := RealMinor(12345, 0, 40)
	if !ok || got != 12345 {
		t.Errorf("RealMinor at 0%% = %d,%v want the input unchanged", got, ok)
	}
	f, ok := Factor(0, 100)
	if !ok || f != 1 {
		t.Errorf("Factor at 0%% = %v,%v want 1,true", f, ok)
	}
}

// An unusable rate reports NOT-OK rather than returning the input, because there
// the caller must not present the result as a real figure.
func TestUnusableRatesRefuseRatherThanPassingThrough(t *testing.T) {
	for _, bad := range []float64{-100, -150, 1001} {
		if _, ok := RealMinor(10000, bad, 10); ok {
			t.Errorf("rate %v was accepted", bad)
		}
		if Valid(bad) {
			t.Errorf("Valid(%v) = true", bad)
		}
	}
	// Deflation IS valid — rare, but real, and clamping it would hide a direction.
	if !Valid(-2) {
		t.Error("mild deflation was rejected")
	}
}

func TestHorizonGuards(t *testing.T) {
	if _, ok := Factor(3, -1); ok {
		t.Error("a negative horizon was accepted")
	}
	if _, ok := Factor(3, MaxYears+1); ok {
		t.Error("an absurd horizon was accepted")
	}
	if _, ok := Factor(3, math.NaN()); ok {
		t.Error("NaN years was accepted")
	}
	// Zero years is now: no deflation at all.
	f, ok := Factor(3, 0)
	if !ok || f != 1 {
		t.Errorf("Factor at 0 years = %v,%v want 1,true", f, ok)
	}
}

// The inverse: what today's price becomes by the time you have saved for it.
func TestNominalMinorIsTheInverse(t *testing.T) {
	future, ok := NominalMinor(3000000, 3, 8)
	if !ok {
		t.Fatal("not ok")
	}
	if future <= 3000000 {
		t.Errorf("a $30,000 kitchen in 8 years = %d, want more than today's price", future)
	}
	// Round-tripping returns roughly the original.
	back, _ := RealMinor(future, 3, 8)
	if math.Abs(float64(back-3000000)) > 2 {
		t.Errorf("round trip = %d, want ~3000000", back)
	}
}

// Erosion states the point without asking anyone to compare two figures.
func TestErosionPct(t *testing.T) {
	got, ok := ErosionPct(3, 20)
	if !ok {
		t.Fatal("not ok")
	}
	if !near(got, 44.63, 0.1) {
		t.Errorf("ErosionPct = %v, want ~44.6", got)
	}
	// Zero rate erodes nothing.
	if e, _ := ErosionPct(0, 50); e != 0 {
		t.Errorf("ErosionPct at 0%% = %v", e)
	}
	// Deflation reports NEGATIVE erosion — a gain in purchasing power — rather
	// than being clamped to zero, which would hide a real direction.
	if e, _ := ErosionPct(-2, 10); e >= 0 {
		t.Errorf("deflation reported erosion of %v, want negative", e)
	}
}

func TestYearsBetweenUsesTheLeapYearAverage(t *testing.T) {
	if got := YearsBetween(365.25); !near(got, 1, 1e-9) {
		t.Errorf("YearsBetween(365.25) = %v, want 1", got)
	}
	// A decade of days is a decade, not a decade plus two days of drift.
	if got := YearsBetween(3652.5); !near(got, 10, 1e-9) {
		t.Errorf("YearsBetween(3652.5) = %v, want 10", got)
	}
}

// The default is an assumption, not a measurement, and callers are expected to
// say so — pinning it here so a change is deliberate.
func TestDefaultRate(t *testing.T) {
	if DefaultRatePct != 3.0 {
		t.Errorf("DefaultRatePct = %v — changing this changes every long-horizon "+
			"figure in the app, so it should be a deliberate edit", DefaultRatePct)
	}
}
