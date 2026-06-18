// Package textutil holds small, pure string helpers shared across CashFlux —
// kept out of the view layer so they can be unit-tested on native Go.
package textutil

import "strings"

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
