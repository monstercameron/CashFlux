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

func TestPluralize(t *testing.T) {
	cases := map[string]string{
		"budget":   "budgets",
		"slider":   "sliders",
		"category": "categories", // the C593 defect: "categorys" shipped
		"day":      "days",       // vowel + y keeps the simple rule
		"box":      "boxes",
		"match":    "matches",
		"dish":     "dishes",
		"class":    "classes",
		// "quizzes" doubles its z — an irregular this rule deliberately does not
		// know. Pinned so the limit is documented rather than discovered.
		"quiz": "quizes",
		"":     "",
	}
	for in, want := range cases {
		if got := Pluralize(in); got != want {
			t.Errorf("Pluralize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	cases := []struct {
		n        int
		singular string
		want     string
	}{
		{1, "budget", "1 budget"},
		{0, "budget", "0 budgets"},
		{2, "budget", "2 budgets"},
		{1, "category", "1 category"},
		{11, "category", "11 categories"},
	}
	for _, c := range cases {
		if got := Plural(c.n, c.singular); got != c.want {
			t.Errorf("Plural(%d, %q) = %q, want %q", c.n, c.singular, got, c.want)
		}
	}
}
