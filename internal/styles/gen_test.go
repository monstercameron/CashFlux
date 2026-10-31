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
		"--control-border:#9a9aa2", "--control-border:#56565c",
		".fctrl-search:focus-within{box-shadow:0 0 0 3px",
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

func TestTransactionToolbarUsesControlBoundary(t *testing.T) {
	resetSheet()
	registerTxnToolbar()
	out := Build()
	for _, want := range []string{
		".filter-panel{", "border:1px solid var(--control-border)",
		".filter-pill{", "border:1px solid var(--control-border)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transaction toolbar CSS missing %q", want)
		}
	}
}

func TestR10TFiltersTriggerKeepsItsWidthWhenBadgeAppears(t *testing.T) {
	resetSheet()
	registerGenerated()
	css := Build()
	start := strings.Index(css, ".filter-toolbar-primary .filters-trigger{")
	if start < 0 {
		t.Fatal("filters trigger rule is missing")
	}
	end := strings.Index(css[start:], "}")
	if end < 0 {
		t.Fatal("filters trigger rule is incomplete")
	}
	rule := css[start : start+end+1]
	for _, want := range []string{"width:7.5rem", "flex:0 0 7.5rem", "box-sizing:border-box"} {
		if !strings.Contains(rule, want) {
			t.Errorf("filters trigger rule %q does not reserve a fixed badge slot (%s)", rule, want)
		}
	}
}
