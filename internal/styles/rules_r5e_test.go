// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR5EDirectionAndSuggestionStyles(t *testing.T) {
	resetSheet()
	registerR5E()
	css := Build()
	for _, want := range []string{
		".txn-dir-toggle > .txn-dir-option.is-pressed, .qa-grid .btn-group > [aria-pressed=\"true\"]{",
		"background:color-mix(in srgb, var(--accent) 16%, var(--bg-elev));",
		"border-color:var(--accent);",
		".txn-edit-category-suggestion{display:inline-flex;align-items:center;justify-content:center;min-height:2rem;",
		".txn-edit-category-suggestion:focus-visible{outline:2px solid var(--accent);outline-offset:2px;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("direction/suggestion CSS missing %q", want)
		}
	}
}

func TestR7FSharedTransactionRowsAndDestructiveDeleteStyles(t *testing.T) {
	resetSheet()
	registerR5E()
	css := Build()
	for _, want := range []string{
		".txn-shared-row{display:grid;grid-template-columns:repeat(2, minmax(0, 1fr));",
		".txn-edit-delete{min-height:2.25rem;padding:0.3rem 0.55rem;color:var(--money-negative);",
		".txn-edit-delete:hover{color:var(--money-negative);background:color-mix(in srgb, var(--money-negative) 12%, transparent);",
		".txn-edit-delete:focus-visible{outline:2px solid var(--money-negative);outline-offset:2px;}",
		".set-foot > .txn-edit-delete{margin-right:auto;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("R7F CSS missing %q", want)
		}
	}
}
