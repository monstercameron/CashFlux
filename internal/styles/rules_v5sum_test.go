// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/contrast"
	"github.com/monstercameron/CashFlux/internal/theme"
)

func TestV5SUMBandIsCompactAndUsesProportionalSixPixelBar(t *testing.T) {
	resetSheet()
	registerUXT1()
	css := Build()
	for _, want := range []string{
		".txn-count-band{display:flex;align-items:center;gap:1rem;width:100%;min-width:0;min-height:44px;max-height:64px;",
		".txn-count-value{font-size:17px;",
		".txn-count-label{font-size:12px;",
		".txn-category-bar{display:flex;width:100%;height:6px;min-height:6px;",
		".txn-category-segment{display:block;flex:0 0 auto;min-width:0;height:6px;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("V5SUM stylesheet missing %q", want)
		}
	}
}

func TestV5SUMPositiveTextUsesAccessibleLightThemeToken(t *testing.T) {
	resetSheet()
	registerUXT1()
	if css := Build(); !strings.Contains(css, ".txn-count-positive{color:var(--positive, var(--up-ink));}") {
		t.Fatalf("positive summary tone must use --positive / --up-ink: %s", css)
	}
	paper, ok := theme.Preset("Paper")
	if !ok {
		t.Fatal("Paper theme preset is missing")
	}
	vars := paper.CSSVars()
	ratio, err := contrast.Ratio(vars["--up-ink"], paper.BgBase)
	if err != nil {
		t.Fatalf("measure light theme positive text contrast: %v", err)
	}
	if ratio < 4.5 {
		t.Fatalf("light theme positive text contrast is %.2f:1, want at least 4.5:1", ratio)
	}
}
