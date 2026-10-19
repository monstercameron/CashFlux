// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR1DFilterSheetIsFixedWidthOverlay(t *testing.T) {
	resetSheet()
	registerGenerated()
	css := Build()
	for _, want := range []string{
		".filter-inline-panel.filter-sheet{",
		"position:fixed",
		"width:min(380px, 100vw)",
		"overflow-y:auto",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("filter sheet CSS does not contain %q", want)
		}
	}
}
