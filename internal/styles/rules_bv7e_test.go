// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBV7EBudgetDetailsUseSharedMetaScale(t *testing.T) {
	resetSheet()
	registerRulesBV7E()
	css := Build()
	for _, selector := range []string{
		".bento-budgets [data-testid^=budget-target-]{font-size:var(--fs-meta);font-weight:400;line-height:1.4;",
		".bento-budgets [data-testid^=budget-free-after-committed-]{font-size:var(--fs-meta);font-weight:400;line-height:1.4;",
	} {
		if !strings.Contains(css, selector) {
			t.Errorf("BV7E stylesheet missing %q", selector)
		}
	}
}
