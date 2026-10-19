// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBR2TBToolbarAndFollowUpRules(t *testing.T) {
	resetSheet()
	registerRulesBR2TB()
	css := Build()
	for _, want := range []string{
		".bud2-tools-menu{min-width:260px;}",
		".bud2-tools-menu .add-item{white-space:nowrap;}",
		".budget-agemoney-why{min-width:24px;min-height:24px;}",
		".budget-issues-chevbtn{min-width:24px;min-height:24px;}",
		".budget-issues-rail.is-followups-only{background:var(--bg-card);border-color:var(--border);color:var(--muted);}",
		"@media (pointer: coarse){.budget-issues-chevbtn{min-width:44px;min-height:44px;}}",
		"@media (pointer: coarse){.budget-agemoney-why{min-width:44px;min-height:44px;}}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BR2TB stylesheet missing %q", want)
		}
	}
}
