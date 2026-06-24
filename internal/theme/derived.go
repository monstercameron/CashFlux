// SPDX-License-Identifier: MIT

package theme

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/contrast"
)

// warnToken is the semantic "warning" color emitted as --warn. Warning is a fixed
// semantic (amber), legible on both dark and light surfaces, so it isn't derived
// from the palette — the shell can reference var(--warn) regardless of theme.
const warnToken = "#e0a93b"

// mixHex blends two sRGB hex colors, returning a's color at t=0 and b's at t=1
// (t clamped to [0,1]), formatted as "#rrggbb". If either color can't be parsed it
// returns a unchanged, so a malformed token degrades to a real color rather than an
// empty value.
func mixHex(a, b string, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ar, ag, ab, err := contrast.ParseHex(a)
	if err != nil {
		return a
	}
	br, bg, bb, err := contrast.ParseHex(b)
	if err != nil {
		return a
	}
	lerp := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return fmt.Sprintf("#%02x%02x%02x", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

// bgElev is the elevated surface one step above the card — the card lifted slightly
// toward the text color — for popovers, hover, and raised chrome.
func (t Theme) bgElev() string { return mixHex(t.BgCard, t.Text, 0.06) }

// derivedVars are the extra CSS tokens the shell needs that aren't stored Theme
// fields: an elevated surface (--bg-elev), a fainter text (--text-faint), a dimmed
// accent (--accent-dim), a semantic warn (--warn), and a --danger alias for Down
// (mirroring the --bg alias for BgBase). They're derived from the theme's own
// tokens, so any theme — built-in or custom — gets sensible values with no
// migration. CSSVars emits these alongside the stored tokens.
func (t Theme) derivedVars() map[string]string {
	return map[string]string{
		"--bg-elev":    t.bgElev(),
		"--text-faint": mixHex(t.TextDim, t.BgBase, 0.40),
		"--accent-dim": mixHex(t.Accent, t.BgBase, 0.45),
		"--warn":       warnToken,
		"--danger":     t.Down,
	}
}
