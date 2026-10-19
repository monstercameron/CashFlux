// SPDX-License-Identifier: MIT

package styles

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestBV7HDesktopBudgetStylesUseThemeTokensAndOneScale(t *testing.T) {
	resetSheet()
	registerBV7H()
	css := Build()
	for _, want := range []string{
		"--fs-meta:12.5px;--fs-body:14px;--fs-section:15px;--fs-title:18px;--fs-figure:28px;",
		".bento-budgets .bud3-attention-link, .bento-budgets .budget-meta-link, .bento-budgets .budget-unbudgeted-all, .bento-budgets .budget-unbudgeted-cta{color:var(--text-dim) !important;}",
		".bento-budgets .budget-crow-figures{column-gap:12px;min-width:260px;flex-wrap:nowrap;white-space:nowrap;font-size:var(--fs-body);}",
		"grid-template-columns:minmax(220px, 1fr) minmax(260px, max-content) minmax(180px, 1.4fr) 110px 72px;",
		".bento-budgets .budget-crow-chip{max-width:100%;min-width:0;flex-shrink:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}",
		".bento-budgets .budget-rollover-detail th, .bento-budgets .budget-rollover-detail td{padding:0.45rem 0.65rem;}",
		".bento-budgets .budget-limit-editform{display:grid;grid-template-columns:minmax(180px, 1fr) auto minmax(180px, 240px) auto auto;align-items:center;gap:0.5rem 0.75rem;padding:0;border:0;border-radius:0;background:transparent;}",
		".bento-budgets .budget-agemoney-head .smart-tip-wrap button, .bento-budgets .budget-agemoney-why{min-width:24px;min-height:24px;}",
		".bento-budgets .budget-issues-toggle{min-height:24px;padding:0.45rem 0.7rem;}",
		"text-transform:none;",
		"@media (min-width: 901px) and (max-width: 1100px)",
		".bento-budgets .budget-crow-amt{white-space:nowrap;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV7H stylesheet is missing %q", want)
		}
	}
	for _, stale := range []string{"#28613f", "rgb(86,86,92)", "font-size:12.5px"} {
		if strings.Contains(css, stale) {
			t.Errorf("BV7H stylesheet retains hard-coded theme or type value %q", stale)
		}
	}
}

func TestBV7HThemeColorsRemainTokenBasedAcrossThemeChanges(t *testing.T) {
	resetSheet()
	registerGenerated()
	registerBV7H()
	css := Build()
	if strings.Contains(css, "bud3-attention-link{color:#") {
		t.Fatal("attention link color is frozen to a resolved color instead of following theme tokens")
	}
	if !strings.Contains(css, "color:var(--text-dim) !important") {
		t.Fatal("attention and metadata text must re-read the active theme's contrast-safe text token")
	}
	for _, theme := range []struct {
		name, foreground, background string
	}{
		{name: "dark", foreground: "#b6b6be", background: "#121214"},
		{name: "light", foreground: "#4e4e55", background: "#ffffff"},
	} {
		if got := contrastBV7H(theme.foreground, theme.background); got < 4.5 {
			t.Errorf("%s theme metadata contrast = %.2f:1, want at least 4.5:1", theme.name, got)
		}
	}
}

func contrastBV7H(foreground, background string) float64 {
	first, second := luminanceBV7H(foreground), luminanceBV7H(background)
	if first < second {
		first, second = second, first
	}
	return (first + 0.05) / (second + 0.05)
}

func luminanceBV7H(color string) float64 {
	channel := func(hex string) float64 {
		value, err := strconv.ParseUint(hex, 16, 8)
		if err != nil {
			panic(err)
		}
		normalized := float64(value) / 255
		if normalized <= 0.04045 {
			return normalized / 12.92
		}
		return math.Pow((normalized+0.055)/1.055, 2.4)
	}
	red, green, blue := channel(color[1:3]), channel(color[3:5]), channel(color[5:7])
	return 0.2126*red + 0.7152*green + 0.0722*blue
}
