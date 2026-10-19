// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBR3RowGridAndBarRulesAtDesktopLaptopAndPhoneWidths(t *testing.T) {
	resetSheet()
	registerBR3Row()
	css := Build()
	for _, want := range []string{
		"grid-template-columns:220px 240px minmax(180px, 1fr) 110px 72px;",
		`grid-template-areas:"name figures bar status actions" "sub sub sub sub sub";`,
		"min-width:180px;height:6px;border-radius:3px;",
		"@media (min-width: 601px) and (max-width: 1100px)",
		"@media (max-width: 600px)",
		`grid-template-areas:"name name status" "figures figures actions" "bar bar bar" "sub sub sub";`,
		"grid-column:1 / -1;",
		".budget-crow:has(.budget-rollover-detail[open]) .budget-rollover-carry{display:none;}",
		"min-height:24px;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BR3 row stylesheet missing %q", want)
		}
	}
}
