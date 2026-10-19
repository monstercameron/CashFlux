// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBC4B6BudgetMetersUseDefinedSurfaceAndRemainVisibleInCompactRows(t *testing.T) {
	resetSheet()
	registerBR3Row()
	registerBud3()
	css := Build()
	if strings.Contains(css, "var(--surface-3)") {
		t.Fatal("budget row uses the undefined --surface-3 token")
	}
	for _, want := range []string{
		".budget-crow-bar{",
		"background:var(--bg-elev);",
		".budget-owner-initial{",
		".budget-crow-bar{grid-column:1 / -1;display:block;width:100%;min-width:0;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("budget row CSS is missing %q", want)
		}
	}
	if strings.Contains(css, "display:none") && strings.Contains(css, ".budget-crow-bar{display:none") {
		t.Fatal("compact budget row hides its meter")
	}
}
