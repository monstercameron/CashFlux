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

// faintText derives the --text-faint tone for dark themes: TextDim pulled toward
// the background as far as possible while still passing WCAG AA (≥4.6:1, a small
// margin over 4.5) against the lightest surface it renders on (the elevated
// card). Starts at the historical 0.28 mix and walks back toward pure TextDim in
// 0.02 steps; if even TextDim itself fails (a pathological custom theme) it
// returns TextDim, never something dimmer.
// accentInk derives the text-safe accent: the accent mixed toward the theme's
// text color (brighter on dark themes, darker on light) in 0.05 steps until it
// passes AA (≥4.6:1) against the elevated surface. Falls back to the text color
// itself if even a full mix can't pass (degenerate custom themes).
func accentInk(accent, text, bgElev string) string {
	// 5.2 (not a bare 4.5) so the ink still clears AA on accent-TINTED chips and
	// selection fills, which sit a step lighter than the elevated surface.
	for mix := 0.0; mix <= 1.0; mix += 0.05 {
		c := mixHex(accent, text, mix)
		if r, err := contrast.Ratio(c, bgElev); err == nil && r >= 5.2 {
			return c
		}
	}
	return text
}

func faintText(textDim, bgBase, bgElev string) string {
	for mix := 0.28; mix >= 0; mix -= 0.02 {
		c := mixHex(textDim, bgBase, mix)
		if r, err := contrast.Ratio(c, bgElev); err == nil && r >= 4.6 {
			return c
		}
	}
	return textDim
}

// derivedVars are the extra CSS tokens the shell needs that aren't stored Theme
// fields: an elevated surface (--bg-elev), a fainter text (--text-faint), a dimmed
// accent (--accent-dim), a semantic warn (--warn), and a --danger alias for Down
// (mirroring the --bg alias for BgBase). They're derived from the theme's own
// tokens, so any theme — built-in or custom — gets sensible values with no
// migration. CSSVars emits these alongside the stored tokens.
func (t Theme) derivedVars() map[string]string {
	m := map[string]string{
		"--bg-elev": t.bgElev(),
		// Dark default: the faint tone must pass AA (4.5:1) on the LIGHTEST surface
		// it sits on — the elevated card (--bg-elev), not just near-black BgBase.
		// The old fixed 0.28 mix landed ~4.06:1 on elevated cards (#67 axe gate);
		// faintText walks the mix back until the ratio clears AA with margin.
		// Light themes override this below (different math).
		"--text-faint": faintText(t.TextDim, t.BgBase, t.bgElev()),
		"--accent-dim": mixHex(t.Accent, t.BgBase, 0.45),
		// --accent-ink: the accent adjusted for use AS TEXT — pulled toward the
		// theme's text color until it clears AA on the elevated surface. The raw
		// accent (#2e8b57 by default) lands ~4.4:1 as small text on dark cards
		// (#67 axe gate); fills/buttons keep using --accent unchanged.
		"--accent-ink": accentInk(t.Accent, t.Text, t.bgElev()),
		"--warn":       warnToken,
		"--danger":     t.Down,
	}
	// --muted and --hover are light-mode surface tokens the stylesheet used to pin
	// with !important because the engine never emitted them (GX14). Emit them here
	// for LIGHT themes only, so ApplyTheme writes a COMPLETE light-derived token set
	// and the !important pins can be relaxed (letting a custom light theme's own
	// surfaces apply). Dark themes keep their prior fallback behavior (unset), so
	// dark mode is unchanged.
	if t.IsLight() {
		m["--muted"] = t.TextDim
		m["--hover"] = mixHex(t.bgElev(), t.Text, 0.05)
		// On a light background a larger mix washes the faint tone toward the bg
		// (~2.8:1 on white — AA fail for sub-labels/legends). Mix only 0.15 toward the
		// (light) background so faint text stays legible (~5.1:1) while still reading
		// fainter than --text-dim.
		m["--text-faint"] = mixHex(t.TextDim, t.BgBase, 0.15)
	}
	return m
}
