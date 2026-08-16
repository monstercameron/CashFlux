// SPDX-License-Identifier: MIT

// Package textutil holds small, pure string helpers shared across CashFlux —
// kept out of the view layer so they can be unit-tested on native Go.
package textutil

import (
	"strconv"
	"strings"
)

// CommaFields splits a comma-separated string into trimmed, non-empty fields,
// preserving order. It returns nil (not an empty slice) when there are no
// fields, so it round-trips cleanly through optional list values (tags, choice
// options, etc.).
func CommaFields(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseFloat parses a (possibly space-padded) decimal string, returning 0 when it
// isn't a valid number — for tolerant numeric form inputs.
func ParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// ParseInt parses a (possibly space-padded) integer string, returning 0 when it
// isn't a valid integer.
func ParseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// FirstNonEmpty returns a when it has non-whitespace content, otherwise b — for
// display fallbacks like "use the description, else a placeholder".
func FirstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// Plural renders "N thing" / "N things" — the count phrase this app puts in
// almost every confirmation, heading and toast.
//
// It lives here rather than in the view layer because the pluralisation rule is
// pure text logic that needs testing: a bare "+s" is right for most of what
// CashFlux counts (budgets, sliders, filters) but produced "11 categorys" in the
// Auto-budget group headings, which is the kind of defect that only shows up on
// screen and only if someone reads it.
func Plural(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(n) + " " + Pluralize(singular)
}

// Pluralize turns an English noun into its plural: consonant + y → -ies, the
// sibilant endings take -es, everything else takes -s. Deliberately not a
// general-purpose inflector — it covers the shapes this app's nouns actually
// have, and an irregular word ("person") should be written out by its caller
// rather than taught to a rule table nobody maintains.
func Pluralize(singular string) string {
	if singular == "" {
		return ""
	}
	if n := len(singular); n > 1 && singular[n-1] == 'y' && !isVowel(singular[n-2]) {
		return singular[:n-1] + "ies"
	}
	for _, suffix := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(singular, suffix) {
			return singular + "es"
		}
	}
	return singular + "s"
}

func isVowel(b byte) bool { return strings.IndexByte("aeiouAEIOU", b) >= 0 }
