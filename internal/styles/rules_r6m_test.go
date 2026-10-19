// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR6MFlattensTransactionRowHeadingsAndKeepsDeleteDivider(t *testing.T) {
	resetSheet()
	registerR6M()
	css := Build()
	for _, want := range []string{
		".txn-row-menu{display:contents;}",
		".txn-row-menu .add-menu-section{display:none;}",
		".txn-row-menu .add-menu-section:has(+ .add-item.danger){display:block;",
		"border-top:1px solid var(--border);",
		"font-size:0;line-height:0;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("R6M menu CSS missing %q", want)
		}
	}
}

func TestR6MLinkChooserUsesCompactModalContentStyles(t *testing.T) {
	resetSheet()
	registerR6M()
	css := Build()
	for _, want := range []string{
		".txn-link-chooser{display:flex;flex-direction:column;gap:.5rem;}",
		".txn-link-chooser .btn{justify-content:flex-start;min-height:2.5rem;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("R6M chooser CSS missing %q", want)
		}
	}
}
