// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBV7CHeadlineWindowUsesSharedMetaTokens(t *testing.T) {
	resetSheet()
	registerBV7C()
	css := Build()
	for _, want := range []string{
		".bento-budgets .bud3-headline-window{font-size:var(--fs-meta);font-weight:400;",
		"line-height:1.35;color:var(--text-dim);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV7C stylesheet missing %q", want)
		}
	}
}
