// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBC5B9BudgetStyleCascadeCleanup(t *testing.T) {
	resetSheet()
	registerBR2MOB()
	registerBR3Row()
	registerBud3()
	registerBR3Head()
	registerBV6H()
	registerBC5B9()
	css := Build()
	for _, forbidden := range []string{
		"display:block !important",
		"padding-bottom:calc(148px + env(safe-area-inset-bottom, 0px)) !important",
		"font-size:12.5px",
	} {
		if strings.Contains(css, forbidden) {
			t.Errorf("budget style still contains obsolete declaration %q", forbidden)
		}
	}
	if !strings.Contains(css, ".budget-crow{grid-template-columns:minmax(180px, 1fr) max-content minmax(160px, 1.2fr) 96px 72px;") {
		t.Fatal("budget compact-row columns are missing")
	}
}
