// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/contrast"
	"github.com/monstercameron/CashFlux/internal/theme"
)

func TestBR2MOBResponsiveRulesAndGreenTextContrast(t *testing.T) {
	resetSheet()
	registerBR2MOB()
	css := Build()
	for _, want := range []string{
		"@media (max-width: 1100px)",
		"grid-template-areas:\"name chip actions\" \"figures figures figures\";",
		".budget-crow-bar{display:block;",
		".budget-crow-head .budget-crow-name{flex:1 1 0;min-width:0;",
		"color:var(--up-ink, var(--accent-ink, var(--up)));",
		".topbar .sample-banner-actions{display:flex;",
		"main.cf-scroll{padding-bottom:calc(148px + env(safe-area-inset-bottom, 0px));",
		".cf-scrolltop{bottom:calc(56px + env(safe-area-inset-bottom, 0px) + 76px);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BR2MOB stylesheet missing %q", want)
		}
	}

	light, ok := theme.Preset("Paper")
	if !ok {
		t.Fatal("Paper theme preset missing")
	}
	dark := theme.Default()
	for _, tc := range []struct {
		name  string
		theme theme.Theme
	}{
		{name: "light", theme: light},
		{name: "dark", theme: dark},
	} {
		vars := tc.theme.CSSVars()
		for _, bg := range []string{tc.theme.BgBase, tc.theme.BgCard} {
			ratio, err := contrast.Ratio(vars["--up-ink"], bg)
			if err != nil {
				t.Fatalf("%s green text contrast: %v", tc.name, err)
			}
			if ratio < 4.5 {
				t.Errorf("%s green text contrast on %s is %.2f:1, want >= 4.5:1", tc.name, bg, ratio)
			}
		}
	}
}
