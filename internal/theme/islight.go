package theme

import "github.com/monstercameron/CashFlux/internal/contrast"

// lightThreshold is the WCAG relative-luminance cutoff above which a base surface
// counts as light. 0.5 splits the 0–1 range at the midpoint, so only genuinely
// bright backgrounds (e.g. the Paper preset) read as light.
const lightThreshold = 0.5

// IsLight reports whether the theme has a light background — i.e. its base surface
// is bright enough that the shell (rail / header / bento) should use its light
// skin rather than the dark default. It is derived from the WCAG relative
// luminance of BgBase, so a theme is "light" purely by virtue of its own tokens
// (no separate light/dark flag to drift out of sync). An unparseable BgBase is
// treated as dark — the app's default — so a malformed custom theme fails safe.
func (t Theme) IsLight() bool {
	l, err := contrast.RelativeLuminance(t.BgBase)
	if err != nil {
		return false
	}
	return l > lightThreshold
}
