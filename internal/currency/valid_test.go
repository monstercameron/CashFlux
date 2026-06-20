package currency

import (
	"sort"
	"testing"
)

func TestValid(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"USD", true},
		{"usd", true},     // case-insensitive
		{"  eur  ", true}, // trimmed
		{"GBP", true},
		{"XYZ", false}, // unknown
		{"", false},    // empty
		{"US", false},  // not a full code
	}
	for _, tc := range tests {
		if got := Valid(tc.code); got != tc.want {
			t.Errorf("Valid(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestList(t *testing.T) {
	list := List()
	if len(list) != len(registry) {
		t.Fatalf("List len = %d, want %d", len(list), len(registry))
	}
	// Sorted by code.
	codes := make([]string, len(list))
	for i, c := range list {
		codes[i] = c.Code
		if c.Name == "" {
			t.Errorf("currency %q has no name", c.Code)
		}
	}
	if !sort.StringsAreSorted(codes) {
		t.Errorf("List is not sorted by code: %v", codes)
	}
	// Every registered code is present and Valid.
	for _, c := range list {
		if !Valid(c.Code) {
			t.Errorf("listed currency %q is not Valid", c.Code)
		}
	}
}
