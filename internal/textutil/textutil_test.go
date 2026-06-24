// SPDX-License-Identifier: MIT

package textutil

import (
	"reflect"
	"testing"
)

func TestParseFloat(t *testing.T) {
	cases := map[string]float64{
		"":        0,
		"  ":      0,
		"abc":     0,
		" 12.5 ":  12.5,
		"-3":      -3,
		"0.0":     0,
		"1000000": 1000000,
	}
	for in, want := range cases {
		if got := ParseFloat(in); got != want {
			t.Errorf("ParseFloat(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseInt(t *testing.T) {
	cases := map[string]int{
		"":      0,
		"x":     0,
		" 42 ":  42,
		"-7":    -7,
		"3.5":   0, // not an integer
		"00012": 12,
	}
	for in, want := range cases {
		if got := ParseInt(in); got != want {
			t.Errorf("ParseInt(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"name", "fallback", "name"},
		{"", "fallback", "fallback"},
		{"   ", "fallback", "fallback"}, // whitespace counts as empty
		{"a", "", "a"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := FirstNonEmpty(c.a, c.b); got != c.want {
			t.Errorf("FirstNonEmpty(%q,%q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestCommaFields(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", nil},
		{"only separators/spaces", " , , ", nil},
		{"trims and drops empties", " a, ,b ,, c ", []string{"a", "b", "c"}},
		{"single value", "solo", []string{"solo"}},
		{"preserves order and inner spaces", "two words, second", []string{"two words", "second"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CommaFields(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("CommaFields(%q) = %#v, want %#v", c.in, got, c.want)
			}
		})
	}
}
