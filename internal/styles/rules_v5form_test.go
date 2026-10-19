// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestV5SplitCategoryHasReadableMinimumWidth(t *testing.T) {
	resetSheet()
	registerV5Form()
	css := Build()
	for _, want := range []string{
		".split-row, .split-column-labels{grid-template-columns:minmax(180px, 1fr) 120px 32px;}",
		".split-row.has-owner, .split-column-labels.has-owner{grid-template-columns:minmax(180px, 1fr) 160px 120px 32px;}",
		".split-category-control .field{width:100%;min-width:180px;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing split form sizing rule %q in %s", want, css)
		}
	}
}

func TestV5SplitRemainderNeutralToneIsMuted(t *testing.T) {
	resetSheet()
	registerV5Form()
	if got, want := Build(), ".hero-stat-sub.split-remainder-neutral{color:var(--muted);}"; !strings.Contains(got, want) {
		t.Fatalf("neutral split remainder style missing: %s", got)
	}
}
