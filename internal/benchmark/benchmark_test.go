// SPDX-License-Identifier: MIT

package benchmark

import (
	"strings"
	"testing"
)

func TestCompareVerdicts(t *testing.T) {
	cases := []struct {
		name        string
		figure      int64
		low, high   int64
		wantVerdict Verdict
		wantDelta   int64
	}{
		{"below", 8000, 10000, 20000, VerdictBelow, -2000},
		{"within-low-edge", 10000, 10000, 20000, VerdictWithin, 0},
		{"within", 15000, 10000, 20000, VerdictWithin, 0},
		{"within-high-edge", 20000, 10000, 20000, VerdictWithin, 0},
		{"above", 26000, 10000, 20000, VerdictAbove, 6000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Compare("car insurance", c.figure, c.low, c.high, "USD", nil)
			if got.Verdict != c.wantVerdict {
				t.Fatalf("verdict = %q, want %q", got.Verdict, c.wantVerdict)
			}
			if got.DeltaMinor != c.wantDelta {
				t.Fatalf("delta = %d, want %d", got.DeltaMinor, c.wantDelta)
			}
		})
	}
}

func TestCompareSwapsReversedRange(t *testing.T) {
	got := Compare("x", 15000, 20000, 10000, "USD", nil)
	if got.LowMinor != 10000 || got.HighMinor != 20000 {
		t.Fatalf("range not normalized: %d-%d", got.LowMinor, got.HighMinor)
	}
	if got.Verdict != VerdictWithin {
		t.Fatalf("verdict = %q", got.Verdict)
	}
}

func TestFormatIncludesSourceAndAssumptions(t *testing.T) {
	c := Compare("car insurance", 26000, 10000, 20000, "USD", []string{"Florida", "full coverage", " "})
	out := c.Format("https://example.com/rates")
	for _, want := range []string{"Your car insurance", "260.00 USD", "Typical range", "above typical", "60.00 USD over", "https://example.com/rates", "Florida", "full coverage"} {
		if !strings.Contains(out, want) {
			t.Fatalf("format missing %q:\n%s", want, out)
		}
	}
}

func TestFormatHonestWhenNoSourceOrAssumptions(t *testing.T) {
	c := Compare("groceries", 40000, 30000, 50000, "USD", nil)
	out := c.Format("")
	if !strings.Contains(out, "none cited") {
		t.Fatalf("missing no-source honesty: %s", out)
	}
	if !strings.Contains(out, "none stated") {
		t.Fatalf("missing no-assumptions honesty: %s", out)
	}
	if !strings.Contains(out, "within the typical range") {
		t.Fatalf("verdict text wrong: %s", out)
	}
}
