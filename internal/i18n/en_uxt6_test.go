// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestUXT6ReviewCopyAndFilterSourceLabel(t *testing.T) {
	for key, want := range map[string]string{
		"reviewInbox.title":           "Review",
		"reviewInbox.categoriesTitle": "Review categories",
		"reviewInbox.countLine":       "%d charges from %d merchants",
		"transactions.filterSource":   "Source",
		"review.tierLook":             "Suggestions to check",
		"review.legendMid":            "Suggestions to check",
	} {
		if got := english[key]; got != want {
			t.Errorf("English %q = %q, want %q", key, got, want)
		}
	}
}
