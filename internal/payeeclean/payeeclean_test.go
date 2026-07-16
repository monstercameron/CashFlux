// SPDX-License-Identifier: MIT

package payeeclean

import "testing"

func TestSuggest(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"", ""},
		{"   ", ""},
		{"SQ *BLUE BOTTLE COFFEE", "Blue Bottle Coffee"},
		{"TST* THE COFFEE BAR #123", "The Coffee Bar"},
		{"PP*NETFLIX", "Netflix"},
		{"AMZN*MKTP", "Amzn Mktp"},
		{"WHOLEFDS MKT 10259", "Wholefds Mkt"},
		{"STARBUCKS STORE #04821", "Starbucks"},
		{"UBER   EATS", "Uber Eats"},
		{"Trader Joe's", "Trader Joe's"}, // already clean, mixed case → unchanged
		{"PAYPAL *SPOTIFY", "Spotify"},
		{"COSTCO WHSE PORTLAND OR", "Costco Whse"},
	}
	for _, c := range cases {
		if got := Suggest(c.raw); got != c.want {
			t.Errorf("Suggest(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestSuggestNeverBlanks(t *testing.T) {
	// A string that is entirely "noise" must still return something, never "".
	if got := Suggest("####"); got == "" {
		t.Errorf("Suggest(%q) blanked the name", "####")
	}
}

func TestWouldChange(t *testing.T) {
	if !WouldChange("SQ *BLUE BOTTLE") {
		t.Error("WouldChange should be true for a dirty descriptor")
	}
	if WouldChange("Blue Bottle") {
		t.Error("WouldChange should be false for an already-clean name")
	}
	if WouldChange("") {
		t.Error("WouldChange should be false for empty")
	}
}
