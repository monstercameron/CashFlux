// SPDX-License-Identifier: MIT

package theme

import "github.com/monstercameron/CashFlux/internal/prefs"

// darkBase and lightBase mirror the live CSS custom properties shipped in
// web/index.html so a Theme migrated from the legacy display preferences
// reproduces the app's current appearance exactly — applying it is a visual
// no-op until the user starts editing. The accent, scale, and density come from
// the user's saved prefs; everything else matches the candidate-C palette.

func darkBase() Theme {
	return Theme{
		Name:        "Custom",
		BgBase:      "#0e0e0f",
		BgCard:      "#121214",
		Border:      "#2a2a2c",
		Text:        "#f4f4f5",
		TextDim:     "#ababb3",
		Accent:      "#2e8b57",
		Up:          "#54b884",
		Down:        "#d8716f",
		Radius:      0,
		FontUI:      "Inter",
		FontDisplay: "Fraunces",
		Scale:       1.0,
		Density:     Comfortable,
		IconStroke:  1.6,
	}
}

func lightBase() Theme {
	return Theme{
		Name:        "Custom",
		BgBase:      "#f7f6f3",
		BgCard:      "#ffffff",
		Border:      "#e4e2dd",
		Text:        "#1c1c1e",
		TextDim:     "#56565c",
		Accent:      "#2e8b57",
		Up:          "#1f8a52",
		Down:        "#b3322f",
		Radius:      0,
		FontUI:      "Inter",
		FontDisplay: "Fraunces",
		Scale:       1.0,
		Density:     Comfortable,
		IconStroke:  1.6,
	}
}

// FromPrefs upgrades the legacy display preferences into a full Theme — the
// migration path for B20's unified appearance engine. It picks the dark or light
// surface palette to match p.Theme, then overlays the user's accent color,
// display scale (percent → multiplier), and density. A "system" theme can't be
// resolved here (this is pure Go with no access to the OS color scheme), so it
// falls back to the dark palette; the wasm layer resolves "system" to a concrete
// light/dark value before calling. The result reproduces today's appearance, so
// switching the app onto the theme engine changes nothing the user can see until
// they edit a token.
func FromPrefs(p prefs.Prefs) Theme {
	p = p.Normalize()

	base := darkBase()
	if p.Theme == prefs.ThemeLight {
		base = lightBase()
	}
	base.Accent = p.Accent
	base.Scale = p.ScaleFraction()
	if p.Compact {
		base.Density = Compact
	}
	return base
}
