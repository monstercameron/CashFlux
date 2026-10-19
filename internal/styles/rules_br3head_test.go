// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBR3HeadBudgetTypeScaleAndControls(t *testing.T) {
	resetSheet()
	registerBR3Head()
	css := Build()
	for _, want := range []string{
		".bento-budgets .bud3-headline-figure{font-size:28px;font-weight:600;",
		".bento-budgets h1, .bento-budgets .budlist-title{font-size:20px;font-weight:600;",
		".bento-budgets h2, .bento-budgets .brc-head{font-size:15px;font-weight:600;",
		".bento-budgets .bud3-headline-secondary{font-size:15px;font-weight:400;",
		".bento-budgets .bud3-headline-meter{position:relative;height:6px;min-height:6px;width:100%;",
		".bento-budgets .budget-crow-name, .bento-budgets .budget-name{font-size:14px;font-weight:600;",
		".bento-budgets .budget-crow-amt, .bento-budgets .budget-crow-left, .bento-budgets .budget-card-loader-figs{font-size:14px;font-weight:500;font-variant-numeric:tabular-nums;",
		".bento-budgets .budget-sub, .bento-budgets .budget-crow-chip, .bento-budgets .bud-set-lbl, .bento-budgets .bud-set-hint{font-size:12.5px;font-weight:400;",
		".bento-budgets .budget-cat-row .cf-picker-trigger::after{content:\"⌄\";",
		"@media (max-width: 720px){.bento-budgets .budlist-head{max-height:96px;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BR3HEAD stylesheet missing %q", want)
		}
	}
}
