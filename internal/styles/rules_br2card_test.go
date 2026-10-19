// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBR2CardFiguresAndPaceStayInNormalFlow(t *testing.T) {
	resetSheet()
	registerBR2Card()
	css := Build()
	for _, want := range []string{
		".bento-budgets .budget-card-loader-figs{position:static;height:auto;min-height:0;",
		".bento-budgets .budget-card-loader{position:relative;height:5px;min-height:5px;",
		".bento-budgets .budget-pace-legend{display:block;position:relative;",
		".bento-budgets .budget-todos-head{width:fit-content;min-width:0;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BR2CARD CSS missing %q", want)
		}
	}
}
