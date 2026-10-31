// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV7FFilterSheetHasOneScrollOwnerAndFixedChrome(t *testing.T) {
	resetSheet()
	registerGenerated()
	registerUXT6()
	registerTV7F()
	css := Build()
	for _, want := range []string{
		".filter-inline-panel.filter-sheet{overflow:hidden;}",
		".filter-inline-panel.filter-sheet .filter-inline-header, .filter-inline-panel.filter-sheet .filter-sheet-footer{flex:0 0 auto;}",
		".filter-inline-panel.filter-sheet .filter-inline-body{flex:1 1 0;min-height:0;overflow-y:auto;}",
		".filter-inline-panel.filter-sheet .filter-search-options, .filter-inline-panel.filter-sheet .filter-pills{max-height:none;overflow:visible;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("TV7F CSS missing %q", want)
		}
	}
}

func TestTV7FBulkControlsShareThirtySixPixelHeight(t *testing.T) {
	resetSheet()
	registerTV7F()
	css := Build()
	want := ".txn-bulkbar-overlay .txn-bulk-primary > *, .txn-bulkbar-overlay > .btn-tool, .txn-bulkbar-overlay .bulk-picker-trigger{box-sizing:border-box;height:36px;min-height:36px;}"
	if !strings.Contains(css, want) {
		t.Fatalf("TV7F bulk height rule missing: %s", want)
	}
}
