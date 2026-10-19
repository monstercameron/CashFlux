// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBV6BMobileBudgetToolbarFlowsBeforeFirstBudget(t *testing.T) {
	resetSheet()
	registerRulesBV6B()
	css := Build()
	for _, want := range []string{
		"@media (max-width: 600px){.budlist-head .budgets-tb{width:100%;max-width:100%;margin:0;min-height:0;}}",
		"@media (max-width: 600px){.budlist-head .budgets-tb .filter-toolbar-actions{display:grid;grid-template-columns:repeat(3, minmax(0, 1fr));align-items:stretch;gap:0.5rem;width:100%;min-width:0;height:auto;min-height:0;margin:0;padding:0;position:static;}}",
		"@media (max-width: 600px){.budlist-head .budgets-tb .filter-toolbar-actions > [data-testid=budgets-add]{grid-column:1 / -1;width:100%;min-height:44px;margin:0;justify-content:center;}}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV6B stylesheet missing %q", want)
		}
	}
}
