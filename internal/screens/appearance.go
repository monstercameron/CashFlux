// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// appearanceThemeWord maps a theme mode to its display word for the hero.
func appearanceThemeWord(t prefs.Theme) string {
	switch t {
	case prefs.ThemeLight:
		return uistate.T("settings.themeLight")
	case prefs.ThemeSystem:
		return uistate.T("settings.themeSystem")
	default:
		return uistate.T("settings.themeDark")
	}
}

// appearanceMotionWord maps a motion preference to its display word.
func appearanceMotionWord(m prefs.Motion) string {
	switch m {
	case prefs.MotionSubtle:
		return uistate.T("settings.motionSubtle")
	case prefs.MotionOff:
		return uistate.T("settings.motionOff")
	default:
		return uistate.T("settings.motionFull")
	}
}

// Appearance is the dedicated appearance screen reachable at /appearance,
// presented in the Understand-surface language: a hero that reads the current
// look back in plain English, then the mode/motion/accent controls and the
// full theme editor as serif sections. All controls write to the same
// cashflux:theme and cashflux:prefs stores as the Settings panel, so
// preferences persist and are consistent regardless of where they were set.
func Appearance() uic.Node {
	prefsAtom := uistate.UsePrefs()
	pr := prefsAtom.Get()
	// The hero reads the APPLIED look (accent from the document root). A theme
	// apply can change it without touching prefs, so subscribe to the shared
	// revision the editor bumps.
	_ = uistate.UseDataRevision().Get()

	savePrefs := func(p prefs.Prefs) {
		uistate.ApplyPrefs(p)
		uistate.PersistPrefs(p)
		prefsAtom.Set(p)
		// A saved custom theme pins the shell to its own luminance, so a mode
		// flip must re-base it (keeping accent/fonts/shape) or it silently loses.
		uistate.SyncThemeToMode(p)
		// Re-derive + apply the theme so the engine's INLINE CSS vars (--text-dim,
		// --text-faint, surfaces…) track the new light/dark mode — exactly as boot
		// does (app.go: ApplyTheme(LoadTheme())). Without this, toggling to Light
		// only flipped data-theme while boot's dark --text-dim (#ababb3) stayed inline
		// and beat the [data-theme="light"] stylesheet, so dim text rendered at ~2.3:1
		// on white (WCAG-AA fail). LoadTheme returns a saved custom theme unchanged, or
		// re-derives DefaultTheme from the just-persisted prefs.
		uistate.ApplyTheme(uistate.LoadTheme())
	}

	// Read the accent from the document root, not prefs: two systems write
	// --accent (the prefs swatch and the theme engine), and a preset's accent
	// never lands in prefs — the hero chip would keep reading the old swatch.
	accent := uistate.CurrentAccent()

	// ── Hero: the current look, read back in plain English. ────────────────────
	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-appearance-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), uistate.T("appearance.eyebrow")),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("settings.appearance")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), appearanceThemeWord(pr.Theme)),
			),
		),
		Div(css.Class("debt-chips"),
			rptChip(uistate.T("settings.motion"), appearanceMotionWord(pr.Motion), ""),
			rptChip(uistate.T("settings.accent"), accent, ""),
		),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "appearance-takeaway"),
			uistate.T("appearance.takeaway", appearanceThemeWord(pr.Theme), appearanceMotionWord(pr.Motion))),
	)

	// ── Mode & motion & accent controls (unchanged mechanics). ─────────────────
	controls := Div(
		// Theme mode — Dark / Light / System. C318: the Segmented itself carries the
		// accessible group name (role="radiogroup" + aria-label).
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.appearance")),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.appearance"),
				Options: []ui.SegOption{
					{Value: string(prefs.ThemeDark), Label: uistate.T("settings.themeDark")},
					{Value: string(prefs.ThemeLight), Label: uistate.T("settings.themeLight")},
					{Value: string(prefs.ThemeSystem), Label: uistate.T("settings.themeSystem")},
				},
				Selected: string(pr.Theme),
				OnSelect: func(v string) {
					p := prefsAtom.Get()
					p.Theme = prefs.Theme(v)
					savePrefs(p)
				},
			}),
		),
		// Motion / WONDER.
		Div(css.Class("toggle-row", tw.Mt2),
			Span(uistate.T("settings.motion")),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.motion"),
				Options: []ui.SegOption{
					{Value: string(prefs.MotionFull), Label: uistate.T("settings.motionFull")},
					{Value: string(prefs.MotionSubtle), Label: uistate.T("settings.motionSubtle")},
					{Value: string(prefs.MotionOff), Label: uistate.T("settings.motionOff")},
				},
				Selected: string(pr.Motion),
				OnSelect: func(v string) {
					p := prefsAtom.Get()
					p.Motion = prefs.Motion(v)
					savePrefs(p)
				},
			}),
		),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.motionHint")),
		// Accent color swatch picker.
		Div(css.Class("toggle-row", tw.Mt2), Attr("role", "group"), Attr("aria-label", uistate.T("settings.accent")),
			Span(uistate.T("settings.accent")),
			ui.SwatchPicker(ui.SwatchPickerProps{
				Colors:   []string{"#2e8b57", "#cfa14e", "#7c83ff", "#d8716f"},
				Selected: pr.Accent,
				OnSelect: func(c string) {
					p := prefsAtom.Get()
					p.Accent = c
					savePrefs(p)
				},
			}),
		),
	)

	return Div(css.Class("bento bento-sys"),
		rptTile("appearance-hero", "1 / span 4", rptSection("", uistate.T("appearance.heroTitle"), nil, heroBody)),
		rptTile("appearance-mode", "1 / span 4",
			rptSection("sec-appearance-mode", uistate.T("appearance.modeTitle"), nil, controls)),
		// Theme editor: presets, color tokens, typography, density, banner.
		rptTile("appearance-editor", "1 / span 4",
			rptSection("sec-appearance-editor", uistate.T("appearance.editorTitle"), nil,
				uic.CreateElement(ThemeEditor))),
	)
}
