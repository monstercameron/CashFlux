// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestPageHeadlineSharedStyles(t *testing.T) {
	resetSheet()
	registerPageHeadline()
	css := Build()
	for _, want := range []string{
		".page-headline-figure{font-size:var(--fs-figure);font-weight:600;",
		".page-headline-aside{font-size:var(--fs-meta);font-weight:400;",
		".page-headline-secondary{display:inline-flex;",
		".page-headline .cf-bar-pace{position:absolute;",
		".page-headline-attention:empty{display:none;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("page headline stylesheet missing %q", want)
		}
	}
}
