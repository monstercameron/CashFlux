// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR7ZBulkBarViewportLayerHasNoTileChrome(t *testing.T) {
	resetSheet()
	registerR7Z()
	css := Build()
	for _, want := range []string{
		`html .w[data-widget="txn-bulkbar"], html .w[data-widget="txn-bulkbar"]:hover{`,
		"background:transparent !important;",
		"border:0 !important;",
		"box-shadow:none !important;",
		"outline:none !important;",
		"padding:0 !important;",
		"backdrop-filter:none !important;",
		"overflow:visible !important;",
		"transform:none !important;will-change:auto !important;transition:none !important;animation:none !important;",
		`html .w[data-widget="txn-bulkbar"]::before, html .w[data-widget="txn-bulkbar"]::after{content:none !important;display:none !important;}`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("R7Z transparent bulk bar layer missing %q", want)
		}
	}
}

func TestR7ZOpenRowMenuPaintsAboveSiblingsAndFitsActionsCell(t *testing.T) {
	resetSheet()
	registerGenerated()
	registerR6M()
	registerR7Z()
	css := Build()
	for _, want := range []string{
		".txn-table tbody tr.row:has(.overflow-menu-open){position:relative;z-index:60;}",
		".txn-table tbody tr.row:has(.overflow-menu-open) > td.td-actions{position:relative;z-index:61;overflow:visible;}",
		".txn-row-menu{display:contents;}",
		".txn-table tbody tr.row{height:44px;max-height:44px;}",
		".txn-table .td-actions{white-space:nowrap;",
		".add-menu{position:absolute;left:0;right:auto;top:calc(100% + 6px);",
		".add-menu.open-up{top:auto;bottom:calc(100% + 6px);}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("R7Z row menu stacking/layout CSS missing %q", want)
		}
	}
}
