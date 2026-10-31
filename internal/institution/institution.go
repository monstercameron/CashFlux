// SPDX-License-Identifier: MIT

// Package institution holds the pure rules for the institution directory: which
// colour an institution is drawn in, how a colour is validated on save, and how
// the directory is ordered and offered as quick picks.
package institution

import (
	"errors"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// DefaultSwatchColor is the colour used when an institution has none set.
const DefaultSwatchColor = "#7c83ff"

// ErrInvalidColor reports a colour that is not a plain #rgb or #rrggbb value.
var ErrInvalidColor = errors.New("institution: colour must be #rgb or #rrggbb")

// IsHexColor reports whether s is a #rgb or #rrggbb colour.
func IsHexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// NormalizeColor validates a colour typed or picked for an institution and returns
// it trimmed. A blank colour is valid and stays blank, meaning the default swatch.
// The colour is stored data that ends up in a generated style, so anything other
// than a plain hex value is rejected with ErrInvalidColor.
func NormalizeColor(raw string) (string, error) {
	c := strings.TrimSpace(raw)
	if c == "" {
		return "", nil
	}
	if !IsHexColor(c) {
		return "", ErrInvalidColor
	}
	return c, nil
}

// SwatchColor returns the colour an institution's chip or stripe is drawn in.
// A stored colour that is not a plain hex value falls back to DefaultSwatchColor.
func SwatchColor(in domain.Institution) string {
	if c := strings.TrimSpace(in.Color); IsHexColor(c) {
		return c
	}
	return DefaultSwatchColor
}

// SortByName orders institutions by trimmed name, case-insensitively, in place.
// Institutions with a blank name sort last, in their incoming order, whatever
// placeholder the view shows for them.
func SortByName(insts []domain.Institution) {
	sort.SliceStable(insts, func(i, j int) bool {
		a, b := insts[i].CleanName(), insts[j].CleanName()
		if (a == "") != (b == "") {
			return b == ""
		}
		return domain.NameLess(a, b)
	})
}

// Choices returns the names of the directory's institutions, trimmed, without
// case-insensitive duplicates and ordered with domain.NameLess, for quick picks in
// a prompt. Of two spellings that differ only in case, the one that sorts first
// by plain string comparison is kept, so the result does not depend on map order.
func Choices(insts map[string]domain.Institution) []string {
	names := make([]string, 0, len(insts))
	for _, in := range insts {
		names = append(names, in.CleanName())
	}
	sort.Strings(names)
	return domain.UniqueNames(names)
}
