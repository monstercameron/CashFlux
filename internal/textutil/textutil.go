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
