// SPDX-License-Identifier: MIT

package contradict

// Visible returns the findings the viewer has not dismissed, in their original
// order. dismissed is keyed by Finding.Key, which stays the same as the amounts
// change, so a dismissal holds until the contradiction itself goes away. A nil or
// empty set hides nothing.
func Visible(fs []Finding, dismissed map[string]bool) []Finding {
	out := make([]Finding, 0, len(fs))
	for _, f := range fs {
		if !dismissed[f.Key] {
			out = append(out, f)
		}
	}
	return out
}
