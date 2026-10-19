// SPDX-License-Identifier: MIT

package styles

import (
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/contrast"
	"github.com/monstercameron/CashFlux/internal/theme"
)

func TestUXT2RulesUsePageTypeScaleAndSurfaces(t *testing.T) {
	resetSheet()
	registerUXT2()
	css := Build()
	for _, want := range []string{
		`.bento > .w[data-widget="txn-toolbar"]{background:transparent;border:0;border-radius:0;box-shadow:none;}`,
		`.bento > .w[data-widget="txn-table"]{background:var(--bg-card);border:1px solid var(--border);border-radius:12px;box-shadow:none;}`,
		`body:has(.bento-ledger) h1{font-family:var(--font-display);font-size:24px;font-weight:600;}`,
		`font-size:24px;font-weight:600;`,
		`font-size:17px;font-weight:600;`,
		`font-size:14px;font-weight:400;`,
		`font-size:12px;font-weight:600;`,
		`.filter-chips-lead, .bento > .w[data-widget="txn-table"] .std-pager-size-label{font-size:12px;font-weight:400;color:var(--text-dim);}`,
		`color-mix(in srgb, var(--accent) 8%, var(--bg-card))`,
		`inset 0 0 0 2px var(--text)`,
		`color:var(--up-ink);`,
		`border-radius:999px;`,
		`border-radius:6px;`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("UXT2 stylesheet missing %q", want)
		}
	}
	for _, forbidden := range []string{"font-size:9px", "font-size:9.6px", "font-size:10px", "font-size:11px", "font-size:14.5px", "font-weight:500", "font-weight:700"} {
		if strings.Contains(css, forbidden) {
			t.Errorf("UXT2 stylesheet contains disallowed declaration %q", forbidden)
		}
	}
	if strings.Contains(css, ".txn-scheduled-pill{border:") {
		t.Error("scheduled pill retains a nested outline")
	}
}

func TestUXT2SelectedRowTextContrastInBothThemes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		theme theme.Theme
	}{
		{name: "dark", theme: theme.Default()},
		{name: "light", theme: paperTheme(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vars := tc.theme.CSSVars()
			selected := mixUXT2(tc.theme.Accent, tc.theme.BgCard, 0.08)
			foregrounds := map[string]string{
				"primary": tc.theme.Text,
				"muted":   tc.theme.TextDim,
				"expense": tc.theme.Down,
				"income":  vars["--up-ink"],
			}
			for label, fg := range foregrounds {
				ratio, err := contrast.Ratio(fg, selected)
				if err != nil {
					t.Fatalf("%s contrast: %v", label, err)
				}
				if ratio < 4.5 {
					t.Errorf("%s text on selected %s row is %.2f:1; want at least 4.5:1", label, tc.name, ratio)
				}
			}
		})
	}
}

func paperTheme(t *testing.T) theme.Theme {
	t.Helper()
	light, ok := theme.Preset("Paper")
	if !ok {
		t.Fatal("Paper theme preset missing")
	}
	return light
}

func mixUXT2(foreground, background string, amount float64) string {
	fr, fg, fb, err := contrast.ParseHex(foreground)
	if err != nil {
		panic(fmt.Sprintf("invalid foreground color: %v", err))
	}
	br, bg, bb, err := contrast.ParseHex(background)
	if err != nil {
		panic(fmt.Sprintf("invalid background color: %v", err))
	}
	mix := func(a, b uint8) uint8 { return uint8(float64(a)*amount + float64(b)*(1-amount) + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x", mix(fr, br), mix(fg, bg), mix(fb, bb))
}
