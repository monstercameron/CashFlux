// Package learntally provides an on-device payee→category correction tally.
// It records how many times a given payee has been assigned to each category,
// and surfaces a suggestion when the evidence crosses a configurable threshold.
// The tally type is a plain exported map so callers can JSON-marshal it for
// persistence without any additional glue code.
package learntally

import (
	"strings"
	"unicode"
)

// DefaultMinCount is the default minimum correction count before a category
// is suggested for a payee.
const DefaultMinCount = 3

// Tally maps a normalized payee string to a map of categoryID → correction count.
// Because it is a named map type (not a struct), callers can encode/decode it
// directly with encoding/json.
type Tally map[string]map[string]int

// NormalizePayee returns a canonical form of s suitable for use as a tally key:
// all characters are lowercased, leading/trailing whitespace is trimmed, and
// any run of internal whitespace is collapsed to a single ASCII space.
func NormalizePayee(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)

	// Collapse internal whitespace runs to a single space.
	var b strings.Builder
	b.Grow(len(s))
	inSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
		} else {
			b.WriteRune(r)
			inSpace = false
		}
	}
	return b.String()
}

// Increment records one correction of payee → categoryID.
// payee is normalized before storage. The call is a no-op when payee or
// categoryID is empty (after normalization).
func (t Tally) Increment(payee, categoryID string) {
	payee = NormalizePayee(payee)
	if payee == "" || categoryID == "" {
		return
	}
	if t[payee] == nil {
		t[payee] = make(map[string]int)
	}
	t[payee][categoryID]++
}

// TopCategory returns the category with the highest correction count for the
// given payee (payee is normalized before lookup). Ties are broken by choosing
// the lexicographically smallest categoryID for determinism.
// Returns ("", 0) when no data exists for the payee.
func (t Tally) TopCategory(payee string) (categoryID string, count int) {
	cats, ok := t[NormalizePayee(payee)]
	if !ok || len(cats) == 0 {
		return "", 0
	}
	for cat, n := range cats {
		if n > count || (n == count && cat < categoryID) {
			categoryID = cat
			count = n
		}
	}
	return categoryID, count
}

// ShouldSuggest returns the top category and true when its correction count is
// at or above threshold. A threshold ≤ 0 is treated as 1.
// Returns ("", false) when there is no data or the count falls below threshold.
func (t Tally) ShouldSuggest(payee string, threshold int) (categoryID string, ok bool) {
	if threshold <= 0 {
		threshold = 1
	}
	cat, n := t.TopCategory(payee)
	if cat == "" || n < threshold {
		return "", false
	}
	return cat, true
}
