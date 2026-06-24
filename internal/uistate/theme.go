// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/theme"
)

const themeStoreID = "cashflux:theme"

// LoadTheme returns the active appearance theme. If the user has saved a custom
// theme it is loaded from localStorage; otherwise the theme is migrated from the
// legacy display preferences (theme.FromPrefs) so a fresh install reproduces the
// app's default appearance exactly. A "system" preference is resolved to a
// concrete light/dark palette here, where the OS color scheme is readable.
func LoadTheme() theme.Theme {
	if raw := SettingKVGet(themeStoreID); raw != "" {
		if t, err := theme.FromJSON([]byte(raw)); err == nil {
			return t
		}
	}
	return DefaultTheme()
}

// DefaultTheme returns the theme migrated from the current display preferences,
// ignoring any saved custom theme — the target of the editor's "reset to
// default". A "system" preference is resolved to a concrete light/dark palette.
func DefaultTheme() theme.Theme {
	p := loadPrefs()
	p.Theme = resolvePrefsTheme(p.Theme)
	return theme.FromPrefs(p)
}

// PersistTheme saves the active theme to localStorage so it survives reloads.
func PersistTheme(t theme.Theme) {
	data, err := t.ToJSON()
	if err != nil {
		return
	}
	SettingKVSet(themeStoreID, string(data))
}

// ApplyTheme reflects a theme's design tokens onto the document root as CSS
// custom properties, so the stylesheet repaints to match. It writes every var
// from CSSVars() plus a --bg alias (the legacy stylesheet paints the app
// background from --bg, while the engine names it --bg-base), so editing the
// background actually takes effect. Call it on boot and whenever the theme
// changes. With the migrated default theme every value equals the stylesheet's
// own default, so the first application is a no-op the user can't see.
func ApplyTheme(t theme.Theme) {
	root := js.Global().Get("document").Get("documentElement")
	if root.IsNull() || root.IsUndefined() {
		return
	}
	style := root.Get("style")
	for k, v := range t.CSSVars() {
		style.Call("setProperty", k, v)
	}
	style.Call("setProperty", "--bg", t.BgBase)
	// The theme owns density: the stylesheet keys compact spacing off the
	// data-density attribute, and --ui-scale (set above via CSSVars) drives the
	// zoom. This makes the theme the single source of truth for both (the legacy
	// prefs no longer apply them).
	density := string(t.Density)
	if density == "" {
		density = string(theme.Comfortable)
	}
	root.Call("setAttribute", "data-density", density)

	// The theme also owns the shell skin (C69). The hand-written
	// [data-theme="light"] stylesheet override re-skins the rail / header / bento,
	// but it only fires off the data-theme attribute — which the engine never set,
	// so a light preset (Paper) painted light cards inside a still-dark shell.
	// Derive data-theme from the theme's own luminance so any light theme lights the
	// shell. Boot applies the theme after prefs (app.go), making this the
	// authoritative writer, and the migrated default theme matches the prefs value
	// so nothing flips for existing dark/light users.
	shell := "dark"
	if t.IsLight() {
		shell = "light"
	}
	root.Call("setAttribute", "data-theme", shell)
}

// resolvePrefsTheme collapses the "system" theme preference to a concrete
// light or dark value by consulting the OS color scheme, so a migrated theme
// has fixed surfaces. Concrete preferences pass through unchanged.
func resolvePrefsTheme(t prefs.Theme) prefs.Theme {
	if t != prefs.ThemeSystem {
		return t
	}
	m := js.Global().Call("matchMedia", "(prefers-color-scheme: light)")
	if !m.IsNull() && !m.IsUndefined() && m.Get("matches").Bool() {
		return prefs.ThemeLight
	}
	return prefs.ThemeDark
}
