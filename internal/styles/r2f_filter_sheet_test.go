// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR2FFilterSheetKeepsPanelSingleColumnAndRangesReadable(t *testing.T) {
	resetSheet()
	registerGenerated()
	css := Build()
	for _, want := range []string{
		".filter-inline-panel.filter-sheet{position:static",
		"height:100%",
		"width:100%",
		".filter-inline-panel.filter-sheet .filter-inline-body{flex:1 1 0;min-height:0;overflow-y:auto",
		".filter-sheet-footer{display:flex",
		"border-top:1px solid var(--border)",
		".filter-inline-panel.filter-sheet .filter-panel{border:0",
		".filter-inline-panel.filter-sheet .filter-groups{display:flex;flex-direction:column",
		".filter-inline-panel.filter-sheet .filter-group-label{",
		".filter-inline-panel.filter-sheet .txn-presets-label{",
		"text-transform:none",
		".filter-inline-panel.filter-sheet .filter-amount-prefix{position:absolute",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("filter sheet CSS does not contain %q", want)
		}
	}

	bodyRule := filterSheetCSSRule(t, css, ".filter-inline-panel.filter-sheet .filter-inline-body")
	if !strings.Contains(bodyRule, "overflow-y:auto") {
		t.Fatalf("filter sheet body is not its scroll area: %q", bodyRule)
	}
	for _, rule := range strings.Split(css, "}") {
		if !strings.Contains(rule, ".filter-inline-panel.filter-sheet") || strings.Contains(rule, ".filter-inline-body{") {
			continue
		}
		if strings.Contains(rule, "overflow-y:auto") || strings.Contains(rule, "max-height:") {
			t.Errorf("filter sheet descendant creates a nested scroll area: %s}", rule)
		}
	}
}

func filterSheetCSSRule(t *testing.T, css, selector string) string {
	t.Helper()
	start := strings.Index(css, selector+"{")
	if start < 0 {
		t.Fatalf("CSS rule %q not found", selector)
	}
	end := strings.Index(css[start:], "}")
	if end < 0 {
		t.Fatalf("CSS rule %q is not closed", selector)
	}
	return css[start : start+end+1]
}
