// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR6ECompactCategoryActionKeepsEditGridRowsAligned(t *testing.T) {
	resetSheet()
	registerR5E()
	css := Build()
	for _, want := range []string{
		".txn-edit-category-field .txn-cat-new{width:2.25rem;min-width:2.25rem;height:2.25rem;padding:0;justify-content:center;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("compact category control CSS missing %q", want)
		}
	}
}
