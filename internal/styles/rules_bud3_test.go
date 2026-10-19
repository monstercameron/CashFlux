// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBud3BudgetFiguresStayOutsideFillWithReadableContrast(t *testing.T) {
	resetSheet()
	registerBud3()
	css := Build()
	for _, rule := range []string{
		".budget-card-loader-figs{",
		"background:var(--bg-card);",
		"color:var(--text);",
		".budget-crow .budget-crow-left{",
		"grid-template-areas:\"name amount actions\" \"left status status\";",
		"@media (pointer: coarse)",
		"min-height:44px;",
	} {
		if !strings.Contains(css, rule) {
			t.Errorf("BUD3 stylesheet is missing %q", rule)
		}
	}
	for _, theme := range []struct {
		name, text, card string
	}{
		{name: "light", text: "#1c1c1e", card: "#ffffff"},
		{name: "dark", text: "#f4f4f5", card: "#121214"},
	} {
		if ratio := contrastRatio(theme.text, theme.card); ratio < 4.5 {
			t.Errorf("%s budget figure contrast %.2f:1, want at least 4.5:1", theme.name, ratio)
		}
	}
}
