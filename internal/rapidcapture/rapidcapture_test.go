// SPDX-License-Identifier: MIT

package rapidcapture

import "testing"

func TestParse(t *testing.T) {
	got := Parse("coffee 4.50, gas 38, costco 122 split with priya")
	if len(got) != 3 {
		t.Fatalf("want 3 drafts, got %d: %+v", len(got), got)
	}
	if got[0].Label != "coffee" || got[0].Cents != 450 || got[0].Split {
		t.Errorf("draft 0 = %+v", got[0])
	}
	if got[1].Label != "gas" || got[1].Cents != 3800 {
		t.Errorf("draft 1 = %+v", got[1])
	}
	if got[2].Label != "costco" || got[2].Cents != 12200 || !got[2].Split || got[2].SplitWith != "priya" {
		t.Errorf("draft 2 = %+v", got[2])
	}
}

func TestParseAmountForms(t *testing.T) {
	cases := []struct {
		tok   string
		cents int64
		ok    bool
	}{
		{"$4.50", 450, true},
		{"1,000", 100000, true},
		{"38", 3800, true},
		{"12.34", 1234, true},
		{"coffee", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		_, cents, ok := parseAmount(c.tok)
		if ok != c.ok || (ok && cents != c.cents) {
			t.Errorf("parseAmount(%q) = %d,%v want %d,%v", c.tok, cents, ok, c.cents, c.ok)
		}
	}
}

func TestParseSplitVariants(t *testing.T) {
	// "split" alone flags the draft with no named counterpart.
	d := Parse("dinner 60 split")
	if len(d) != 1 || !d[0].Split || d[0].SplitWith != "" || d[0].Label != "dinner" {
		t.Fatalf("split-alone = %+v", d)
	}
	// Multiple names join with a comma.
	d = Parse("rent 1800 split with priya and sam")
	if len(d) != 1 || d[0].SplitWith != "priya, sam" {
		t.Fatalf("split-names = %+v", d)
	}
	// Amount before the label still parses; a clause with no amount is dropped.
	d = Parse("just a note, taxi 15")
	if len(d) != 1 || d[0].Label != "taxi" || d[0].Cents != 1500 {
		t.Fatalf("mixed = %+v", d)
	}
}

func TestParseEmpty(t *testing.T) {
	if got := Parse("   "); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}
