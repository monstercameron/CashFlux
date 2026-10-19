// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestUXT6ReviewAndFilterSheetRules(t *testing.T) {
	resetSheet()
	registerGenerated()
	registerUXT6()
	css := Build()
	for _, want := range []string{
		".rvs-intro-title{margin:0 0 .25rem;font-family:var(--font-sans), sans-serif;font-size:var(--type-17)",
		".rvs-intro-count{margin:0;color:var(--text-dim);font-size:var(--type-14)",
		".rvs-dot.is-look, .rvs-tier.is-look .rvs-tier-mark{background:var(--text-faint);}",
		".rvs-grp.is-focus .rvs-grp-row{outline:2px solid var(--accent);outline-offset:-2px;}",
		".filter-inline-panel.filter-sheet{overflow-y:auto;}",
		".filter-inline-panel.filter-sheet .filter-inline-body{flex:0 0 auto;min-height:auto;overflow:visible;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("UXT6 CSS missing %q", want)
		}
	}
}
