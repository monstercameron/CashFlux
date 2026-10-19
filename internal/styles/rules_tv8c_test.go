// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV8CStylesUseTwoRowsAndPopover(t *testing.T) {
	registerTV8C()
	css := Build()
	for _, want := range []string{
		".txn-count-band{display:grid;grid-template-columns:minmax(0, 1fr) auto;",
		".txn-category-breakdown{grid-column:1 / -1;grid-row:2;",
		".txn-category-popover{position:absolute;z-index:20;",
		".txn-category-popover-row{display:grid;grid-template-columns:minmax(0, 1fr) auto;",
		".txn-count-visible-facts{grid-column:1 / -1;width:100%;}",
		"@media (max-width: 760px){.txn-count-band{grid-template-columns:minmax(0, 1fr);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("TV8C summary style missing %q", want)
		}
	}
}
