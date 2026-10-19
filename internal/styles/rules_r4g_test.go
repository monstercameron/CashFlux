// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR4GMainPaneChromeRules(t *testing.T) {
	resetSheet()
	registerGenerated()
	registerUxbatch4()
	registerR4G()
	css := Build()

	for _, want := range []string{
		"#main.cf-scroll{scrollbar-gutter:stable;}",
		"#main:focus:not(:focus-visible){outline:none;}",
	} {
		if strings.Count(css, want) != 1 {
			t.Errorf("expected exactly one %q rule, found %d", want, strings.Count(css, want))
		}
	}

	stableIndex := strings.Index(css, "#main.cf-scroll{scrollbar-gutter:stable;}")
	mainScrollIndex := strings.Index(css, "main.cf-scroll{scrollbar-width:thin;")
	if stableIndex <= mainScrollIndex {
		t.Errorf("stable gutter rule must follow the generated main scroll rule: indexes %d, %d", stableIndex, mainScrollIndex)
	}

	pointerFocusIndex := strings.Index(css, "#main:focus:not(:focus-visible){outline:none;}")
	focusRingIndex := strings.Index(css, "[tabindex]:focus-visible, [role=\"button\"]:focus-visible")
	if pointerFocusIndex <= focusRingIndex {
		t.Errorf("pointer focus override must follow the generated keyboard focus ring: indexes %d, %d", pointerFocusIndex, focusRingIndex)
	}
	if !strings.Contains(css, "[tabindex]:focus-visible") || !strings.Contains(css, "outline:2px solid var(--accent);outline-offset:2px;border-radius") {
		t.Error("shared keyboard focus ring must remain emitted")
	}
}
