// SPDX-License-Identifier: MIT

package smartengine

import "testing"

func TestPlural(t *testing.T) {
	cases := []struct {
		n    int64
		noun string
		want string
	}{
		{1, "month", "1 month"},
		{3, "month", "3 months"},
		{1, "category", "1 category"},
		{2, "category", "2 categories"},      // consonant+y → ies (not "categorys")
		{97, "entry", "97 entries"},          // consonant+y → ies (not "entrys")
		{0, "bill", "0 bills"},               // zero is plural
		{2, "box", "2 boxes"},                // sibilant x → es
		{2, "charge", "2 charges"},           // regular +s
		{2, "day", "2 days"},                 // vowel+y → +s (not "daies")
		{2, "active goal", "2 active goals"}, // pluralize only the last word
		{1, "household member", "1 household member"},
		{2, "household member", "2 household members"},
	}
	for _, c := range cases {
		if got := plural(c.n, c.noun); got != c.want {
			t.Errorf("plural(%d, %q) = %q, want %q", c.n, c.noun, got, c.want)
		}
	}
}

func TestHmoneyc(t *testing.T) {
	cases := []struct {
		minor int64
		cur   string
		want  string
	}{
		{51937, "USD", "$519"},      // ≥ $100 → rounded whole, grouped
		{543400, "USD", "$5,434"},   // thousands separator
		{562600, "USD", "$5,626"},   // thousands separator
		{19100, "USD", "$191"},      // exactly under grouping, rounded whole
		{99, "USD", "$0.99"},        // small amount keeps cents
		{2250, "USD", "$22.50"},     // under $100 keeps cents
		{-51937, "USD", "-$519"},    // negative
		{1234567, "USD", "$12,346"}, // multi-group rounding
		{0, "USD", "$0.00"},         // zero keeps cents
		{50050, "USD", "$501"},      // rounds 500.50 → 501
	}
	for _, c := range cases {
		if got := hmoneyc(c.minor, c.cur); got != c.want {
			t.Errorf("hmoneyc(%d, %q) = %q, want %q", c.minor, c.cur, got, c.want)
		}
	}
}

func TestGroup(t *testing.T) {
	cases := map[int64]string{
		0: "0", 12: "12", 123: "123", 1234: "1,234",
		12345: "12,345", 123456: "123,456", 1234567: "1,234,567",
	}
	for in, want := range cases {
		if got := group(in); got != want {
			t.Errorf("group(%d) = %q, want %q", in, got, want)
		}
	}
}
