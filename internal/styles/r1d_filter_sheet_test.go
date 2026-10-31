// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR1DFilterSheetFillsItsOverlay(t *testing.T) {
	resetSheet()
	registerGenerated()
	css := Build()
	for _, want := range []string{
		".filter-inline-panel.filter-sheet{",
		"position:static",
		"width:100%",
		"overflow-y:auto",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("filter sheet CSS does not contain %q", want)
		}
	}
}
