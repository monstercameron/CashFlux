// SPDX-License-Identifier: MIT

package styles

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestBV6HTypographyTargetsAndLinkContrast(t *testing.T) {
	resetSheet()
	registerBR3Head()
	registerBud3()
	registerRulesBR2A11Y()
	registerBV6H()
	css := Build()
	for _, want := range []string{
		".bento-budgets{--fs-meta:12.5px;--fs-body:14px;--fs-section:15px;--fs-title:18px;--fs-figure:28px;",
		".bento-budgets h1, .bento-budgets .budlist-title{font-size:var(--fs-title);font-weight:600;",
		".bento-budgets .budget-crow-name, .bento-budgets .budget-name{font-size:var(--fs-body);font-weight:600;",
		".bento-budgets .bud3-attention-link{color:#28613f;",
		".bento-budgets .budget-agemoney-head .smart-tip-wrap button{min-width:32px;min-height:32px;",
		".bento-budgets .budget-agemoney-head .smart-tip-wrap button::after{content:\"Why?\";",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV6H stylesheet is missing %q", want)
		}
	}
	if got := bv6hContrastRatio("#28613f", "#f7f6f3"); got < 4.5 {
		t.Errorf("light-theme link contrast %.2f:1, want at least 4.5:1", got)
	}
}

func bv6hContrastRatio(foreground, background string) float64 {
	fg, bg := bv6hLuminance(foreground), bv6hLuminance(background)
	if bg > fg {
		fg, bg = bg, fg
	}
	return (fg + 0.05) / (bg + 0.05)
}

func bv6hLuminance(color string) float64 {
	var r, g, b int
	if _, err := fmt.Sscanf(color, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return 0
	}
	channel := func(value int) float64 {
		v := float64(value) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}
