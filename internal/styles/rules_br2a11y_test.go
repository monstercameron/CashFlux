// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBR2A11YActionTargets(t *testing.T) {
	resetSheet()
	registerRulesBR2A11Y()
	css := Build()
	for _, selector := range []string{".budget-meta-link", ".budget-unbudgeted-all", ".scope-banner-btn"} {
		if want := selector + "{min-width:24px;min-height:24px;}"; !strings.Contains(css, want) {
			t.Errorf("stylesheet missing %q", want)
		}
		if want := "@media (pointer: coarse){" + selector + "{min-width:44px;min-height:44px;}}"; !strings.Contains(css, want) {
			t.Errorf("stylesheet missing %q", want)
		}
	}
}
