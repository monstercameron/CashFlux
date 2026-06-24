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

// Appearance is the dedicated appearance screen reachable at /appearance.
// It renders the full appearance and theming controls — theme mode, accent
// color, motion, and the complete theme editor — on a calm, full-width routed
// page instead of in the cramped Settings panel (B34). All controls write to
// the same cashflux:theme and cashflux:prefs stores as the Settings panel, so
// preferences persist and are consistent regardless of where they were set.
func Appearance() uic.Node {
	prefsAtom := uistate.UsePrefs()
	pr := prefsAtom.Get()

	savePrefs := func(p prefs.Prefs) {
		uistate.ApplyPrefs(p)
		uistate.PersistPrefs(p)
		prefsAtom.Set(p)
	}

	return Div(css.Class("page-content"),
		// Theme mode — Dark / Light / System.
		// role="group" + aria-label programmatically associates the H4 heading text
		// with the Segmented control so screen readers announce the group name when
		// the control receives focus (WCAG 1.3.1 / 4.1.2).
		Div(Attr("role", "group"), Attr("aria-label", uistate.T("settings.appearance")),
			H4(css.Class("set-label"), uistate.T("settings.appearance")),
			ui.Segmented(ui.SegmentedProps{
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

		// Motion / WONDER — existing toggle-row div gains role="group" + aria-label
		// so the visible "Motion" label is associated with the Segmented control.
		Div(css.Class("toggle-row", tw.Mt2), Attr("role", "group"), Attr("aria-label", uistate.T("settings.motion")),
			Span(uistate.T("settings.motion")),
			ui.Segmented(ui.SegmentedProps{
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

		// Accent color swatch picker — existing toggle-row div gains role="group" +
		// aria-label so the visible "Accent" label is associated with the SwatchPicker.
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

		// Theme editor: presets, color tokens, typography, density, banner
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1.25rem 0 0"})),
		uic.CreateElement(ThemeEditor),
	)
}
