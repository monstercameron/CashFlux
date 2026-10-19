// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR3KBulkBarKeepsPickersVisibleAndLastRowReachable(t *testing.T) {
	resetSheet()
	registerR3K()
	css := Build()
	for _, want := range []string{
		`html .w[data-widget="txn-bulkbar"]{inset:0;width:auto;height:auto;pointer-events:none;overflow:visible;}`,
		`html .w[data-widget="txn-bulkbar"] .wbody-nohead{height:0;padding:0;overflow:visible;}`,
		".txn-bulkbar-overlay{position:absolute;left:max(256px, calc(50vw - 360px));bottom:16px;width:min(960px, calc(100vw - 240px - 32px));max-width:960px;pointer-events:auto;",
		":where(html.cf-rail-c) .txn-bulkbar-overlay{left:max(74px, calc(50vw - 451px));width:min(960px, calc(100vw - 58px - 32px));}",
		"min-height:48px;padding:0 1rem;overflow:visible;flex-wrap:nowrap;white-space:nowrap;background:var(--bg-elev);border:1px solid var(--border);border-radius:var(--radius-lg);",
		`@media (min-width: 1280px){.txn-bulkbar-overlay .add-item[data-testid="bulk-mark-reviewed-menu"], .txn-bulkbar-overlay .add-item[data-testid="bulk-delete-menu"]{display:none;}}`,
		`@media (max-width: 1279px){.txn-bulkbar-overlay .txn-bulk-inline-reviewed, .txn-bulkbar-overlay .txn-bulk-inline-delete{display:none;}}`,
		`@media (max-width: 1279px){.txn-bulkbar-overlay .add-item[data-testid="bulk-mark-reviewed-menu"], .txn-bulkbar-overlay .add-item[data-testid="bulk-delete-menu"]{display:flex;}}`,
		`.txn-bulkbar-overlay .bulk-owner-actions{display:flex;align-items:center;gap:0.5rem;flex:0 1 auto;white-space:nowrap;}`,
		".txn-bulkbar-overlay .bulk-tag-editor{position:absolute;right:10rem;bottom:calc(100% + 8px);z-index:72;",
		".bulk-picker-popover{position:absolute;left:0;bottom:calc(100% + 8px);z-index:72;",
		"main.cf-scroll:has(.txn-has-selection){padding-bottom:80px;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BULK2 bulk bar CSS missing %q", want)
		}
	}
	barStart := strings.Index(css, ".txn-bulkbar-overlay{")
	if barStart < 0 {
		t.Fatal("bulk bar rule is missing")
	}
	barEnd := strings.Index(css[barStart:], "}")
	if barEnd < 0 || strings.Contains(css[barStart:barStart+barEnd], "overflow-x:auto") {
		t.Fatal("bulk bar must not become a horizontal scroll container")
	}
	if strings.Contains(css, ".txn-bulkbar-overlay{position:fixed") || strings.Contains(css, `#txn-bulkbar{width:100vw`) {
		t.Fatal("bulk bar still spans the viewport instead of deriving its bounds from the main pane")
	}
}
