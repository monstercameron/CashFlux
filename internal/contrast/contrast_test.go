// SPDX-License-Identifier: MIT

package contrast

import (
	"math"
	"testing"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestParseHex(t *testing.T) {
	cases := []struct {
		in      string
		r, g, b uint8
		wantErr bool
	}{
		{"#ffffff", 255, 255, 255, false},
		{"000000", 0, 0, 0, false},
		{"#f0a", 255, 0, 170, false}, // shorthand expands
		{"#ABCDEF", 0xAB, 0xCD, 0xEF, false},
		{"#12345", 0, 0, 0, true},  // wrong length
		{"#12345g", 0, 0, 0, true}, // non-hex digit
		{"", 0, 0, 0, true},
	}
	for _, c := range cases {
		r, g, b, err := ParseHex(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("ParseHex(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && (r != c.r || g != c.g || b != c.b) {
			t.Errorf("ParseHex(%q) = %d,%d,%d, want %d,%d,%d", c.in, r, g, b, c.r, c.g, c.b)
		}
	}
}

func TestRelativeLuminanceExtremes(t *testing.T) {
	if l, _ := RelativeLuminance("#000000"); !approx(l, 0, 1e-9) {
		t.Errorf("luminance(black) = %g, want 0", l)
	}
	if l, _ := RelativeLuminance("#ffffff"); !approx(l, 1, 1e-9) {
		t.Errorf("luminance(white) = %g, want 1", l)
	}
}

func TestRatioBlackWhite(t *testing.T) {
	r, err := Ratio("#000000", "#ffffff")
	if err != nil {
		t.Fatalf("Ratio error: %v", err)
	}
	if !approx(r, 21, 1e-6) {
		t.Errorf("Ratio(black,white) = %g, want 21", r)
	}
	// Order doesn't matter.
	r2, _ := Ratio("#ffffff", "#000000")
	if !approx(r, r2, 1e-12) {
		t.Errorf("Ratio is not symmetric: %g vs %g", r, r2)
	}
}

func TestRatioIdentical(t *testing.T) {
	if r, _ := Ratio("#3b82f6", "#3b82f6"); !approx(r, 1, 1e-12) {
		t.Errorf("Ratio of a color with itself = %g, want 1", r)
	}
}

func TestRatioKnownPair(t *testing.T) {
	// #767676 on white is the canonical ~4.54:1 (just passes AA normal).
	r, err := Ratio("#767676", "#ffffff")
	if err != nil {
		t.Fatalf("Ratio error: %v", err)
	}
	if !approx(r, 4.54, 0.05) {
		t.Errorf("Ratio(#767676, white) = %g, want ~4.54", r)
	}
	if !PassesAA(r, false) {
		t.Error("#767676 on white should pass AA normal")
	}
}

func TestPasses(t *testing.T) {
	if !PassesAA(4.5, false) || PassesAA(4.49, false) {
		t.Error("AA normal boundary wrong at 4.5")
	}
	if !PassesAA(3.0, true) || PassesAA(2.99, true) {
		t.Error("AA large boundary wrong at 3.0")
	}
	if !PassesAAA(7.0, false) || PassesAAA(6.9, false) {
		t.Error("AAA normal boundary wrong at 7.0")
	}
	if !PassesAAA(4.5, true) || PassesAAA(4.4, true) {
		t.Error("AAA large boundary wrong at 4.5")
	}
}

func TestRatioErrorsPropagate(t *testing.T) {
	if _, err := Ratio("#zzz", "#ffffff"); err == nil {
		t.Error("expected error for invalid hex")
	}
}
