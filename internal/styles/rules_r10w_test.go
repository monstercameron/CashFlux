// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR10WCategoryAndSubmenuStyles(t *testing.T) {
	resetSheet()
	registerR10W()
	css := Build()
	for _, want := range []string{
		".td-cat-uncategorized{color:var(--text-dim);}",
		".add-submenu{position:absolute;left:calc(100% - 2px);",
		"max-height:calc(100vh - 16px);overflow-y:auto;",
		".add-menu.open-left .add-submenu{left:auto;right:calc(100% - 2px);}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("R10W CSS missing %q", want)
		}
	}
}
