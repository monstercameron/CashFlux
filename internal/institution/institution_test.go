// SPDX-License-Identifier: MIT

package institution

import (
	"errors"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSwatchColorOnlyPassesPlainHex(t *testing.T) {
	cases := []struct {
		name, color, want string
	}{
		{"six digit hex", "#1a2B3c", "#1a2B3c"},
		{"three digit hex", "#abc", "#abc"},
		{"padded hex is trimmed", "  #abc ", "#abc"},
		{"blank falls back", "  ", DefaultSwatchColor},
		{"no hash", "1a2b3c", DefaultSwatchColor},
		{"css injection", "red;}body{display:none", DefaultSwatchColor},
		{"wrong length", "#12", DefaultSwatchColor},
		{"non hex digit", "#ggg", DefaultSwatchColor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SwatchColor(domain.Institution{Color: tc.color}); got != tc.want {
				t.Fatalf("SwatchColor(%q) = %q, want %q", tc.color, got, tc.want)
			}
		})
	}
}

func TestNormalizeColor(t *testing.T) {
	cases := []struct {
		name, in, want string
		wantErr        error
	}{
		{"blank stays blank", "   ", "", nil},
		{"valid hex is trimmed", " #1A2b3C ", "#1A2b3C", nil},
		{"short hex", "#fff", "#fff", nil},
		{"named colour rejected", "red", "", ErrInvalidColor},
		{"style injection rejected", "#fff;}x{", "", ErrInvalidColor},
		{"bad digit rejected", "#12g", "", ErrInvalidColor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeColor(tc.in)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("NormalizeColor(%q) = %q, %v; want %q, %v", tc.in, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestChoicesAreSortedUniqueNames(t *testing.T) {
	cases := []struct {
		name  string
		insts map[string]domain.Institution
		want  []string
	}{
		{
			name: "trims, dedupes and skips blanks",
			insts: map[string]domain.Institution{
				"a": {ID: "a", Name: "Chase"},
				"b": {ID: "b", Name: "  Comcast  "},
				"c": {ID: "c", Name: "Chase"},
				"d": {ID: "d", Name: ""},
			},
			want: []string{"Chase", "Comcast"},
		},
		{
			name: "orders case-insensitively",
			insts: map[string]domain.Institution{
				"a": {ID: "a", Name: "zions"},
				"b": {ID: "b", Name: "Ally"},
				"c": {ID: "c", Name: "chase"},
			},
			want: []string{"Ally", "chase", "zions"},
		},
		{name: "empty directory", insts: nil, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Choices(tc.insts); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Choices = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSortByNameIgnoresCase(t *testing.T) {
	insts := []domain.Institution{{ID: "1", Name: "zions"}, {ID: "2", Name: "Ally"}, {ID: "3", Name: " bmo "}}
	SortByName(insts)
	var ids []string
	for _, in := range insts {
		ids = append(ids, in.ID)
	}
	if want := []string{"2", "3", "1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("SortByName order = %v, want %v", ids, want)
	}
}

func TestChoicesDedupesCaseInsensitivelyAndDeterministically(t *testing.T) {
	insts := map[string]domain.Institution{
		"a": {ID: "a", Name: "Chase"},
		"b": {ID: "b", Name: "chase"},
		"c": {ID: "c", Name: " CHASE "},
		"d": {ID: "d", Name: "Ally"},
	}
	want := []string{"Ally", "CHASE"}
	for i := 0; i < 50; i++ {
		if got := Choices(insts); !reflect.DeepEqual(got, want) {
			t.Fatalf("Choices = %v, want %v on every call", got, want)
		}
	}
}

func TestSortByNameKeepsBlankNamesLastWhateverThePlaceholder(t *testing.T) {
	insts := []domain.Institution{{ID: "blank1", Name: "  "}, {ID: "z", Name: "Zions"}, {ID: "blank2"}, {ID: "a", Name: "ally"}}
	SortByName(insts)
	var ids []string
	for _, in := range insts {
		ids = append(ids, in.ID)
	}
	if want := []string{"a", "z", "blank1", "blank2"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("SortByName order = %v, want %v", ids, want)
	}
}
