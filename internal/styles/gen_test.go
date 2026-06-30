// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

// TestGeneratedHasKeyRules sanity-checks the transpiled design system: key selectors,
// tokens, keyframes, and at-rules are present, and the output is substantial.
func TestGeneratedHasKeyRules(t *testing.T) {
	resetSheet()
	registerGenerated()
	out := Build()

	wants := []string{
		":root{", "--bg:#0e0e0f", "--accent:#2e8b57",
		"*{", "box-sizing:border-box",
		"body{", "#boot{", "@keyframes boot-spin{",
		".btn{", ".bento", ".topbar", ".txn-table",
		"@media print{",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("generated CSS missing %q", w)
		}
	}
	if len(out) < 40_000 {
		t.Errorf("generated CSS unexpectedly small (%d bytes) — parser likely dropped rules", len(out))
	}
	t.Logf("generated CSS: %d bytes, %d rules", len(out), len(sheet))
}
