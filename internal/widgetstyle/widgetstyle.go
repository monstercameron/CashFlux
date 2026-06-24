// SPDX-License-Identifier: MIT

// Package widgetstyle turns a widget's saved styling config (colors, font, weight,
// shape, border, shadow) into the inline CSS that overrides the global theme for
// that tile. It is pure (no syscall/js) so the resolution is unit-testable and
// shared by the dashboard tile, the Widget Manager's editor, and its live preview.
//
// Styling is stored as reserved keys on the existing per-widget widgetcfg.Config
// (alongside the accent key), so it rides the same persistence. A global default
// lives under the reserved widget id "_all"; a per-widget config overrides it
// key-by-key. Only set fields are emitted, so anything left blank inherits the
// global theme.
package widgetstyle

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/widgetcfg"
)

// GlobalID is the reserved widget id whose config holds the default tile style
// applied to every widget that doesn't override it.
const GlobalID = "_all"

// Reserved styling keys on a widgetcfg.Config.
const (
	KeyBg      = "_bg"      // background color (hex)
	KeyText    = "_text"    // text color (hex)
	KeyBorder  = "_border"  // border color (hex)
	KeyBorderW = "_borderW" // border width (px, 0–6)
	KeyRadius  = "_radius"  // corner radius (px, 0–32)
	KeyFont    = "_font"    // font token: sans | display | mono
	KeyWeight  = "_weight"  // font weight: 400 | 500 | 600 | 700
	KeyShadow  = "_shadow"  // shadow token: none | soft | strong
	KeyAccent  = widgetcfg.AccentKey
)

// Keys is every reserved styling key — used to clear a target's style on reset.
var Keys = []string{KeyBg, KeyText, KeyBorder, KeyBorderW, KeyRadius, KeyFont, KeyWeight, KeyShadow, KeyAccent}

var fontFamilies = map[string]string{
	"sans":    "var(--font-sans), system-ui, sans-serif",
	"display": "var(--font-display), 'Fraunces', serif",
	"mono":    "ui-monospace, 'Cascadia Code', 'Consolas', monospace",
}

var shadows = map[string]string{
	"none":   "none",
	"soft":   "0 1px 3px rgba(0,0,0,.12)",
	"strong": "0 8px 24px rgba(0,0,0,.22)",
}

var weights = map[string]bool{"400": true, "500": true, "600": true, "700": true}

// Effective merges a global default config with a per-widget config — the
// per-widget value wins per key — into one styling config holding only the
// reserved style keys.
func Effective(global, perWidget widgetcfg.Config) widgetcfg.Config {
	out := widgetcfg.Config{}
	for _, k := range Keys {
		if v := strings.TrimSpace(perWidget[k]); v != "" {
			out[k] = v
		} else if v := strings.TrimSpace(global[k]); v != "" {
			out[k] = v
		}
	}
	return out
}

// InlineStyle returns the CSS properties to inline on a tile for a styling config.
// Only set, valid fields are emitted; everything else is omitted so the tile keeps
// the global theme value. Invalid values (bad hex, out-of-range numbers, unknown
// tokens) are ignored rather than producing broken CSS.
func InlineStyle(cfg widgetcfg.Config) map[string]string {
	out := map[string]string{}
	if v := hex(cfg[KeyBg]); v != "" {
		out["background-color"] = v
	}
	if v := hex(cfg[KeyText]); v != "" {
		out["color"] = v
	}
	if v := hex(cfg[KeyBorder]); v != "" {
		out["border-color"] = v
		out["border-style"] = "solid"
	}
	if v, ok := px(cfg[KeyBorderW], 0, 6); ok {
		out["border-width"] = v
		out["border-style"] = "solid"
	}
	if v, ok := px(cfg[KeyRadius], 0, 32); ok {
		out["border-radius"] = v
	}
	if fam, ok := fontFamilies[strings.TrimSpace(cfg[KeyFont])]; ok {
		out["font-family"] = fam
	}
	if w := strings.TrimSpace(cfg[KeyWeight]); weights[w] {
		out["font-weight"] = w
	}
	// box-shadow composes an optional accent top strip with an optional drop shadow,
	// so both can be set at once. (CSS custom properties don't survive the inline
	// Style() path, so accent is a visible strip rather than a --accent re-tint.)
	var parts []string
	if a := hex(cfg[KeyAccent]); a != "" {
		parts = append(parts, "inset 0 3px 0 0 "+a)
	}
	shadowTok := strings.TrimSpace(cfg[KeyShadow])
	shadowVal, shadowSet := shadows[shadowTok]
	if shadowSet && shadowVal != "none" {
		parts = append(parts, shadowVal)
	}
	if len(parts) > 0 {
		out["box-shadow"] = strings.Join(parts, ", ")
	} else if shadowSet {
		out["box-shadow"] = "none" // explicit "None" shadow, no accent
	}
	return out
}

// hex returns a normalized #rrggbb (or #rgb) color when valid, else "".
func hex(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s[0] != '#' {
		return ""
	}
	body := s[1:]
	if len(body) != 3 && len(body) != 6 {
		return ""
	}
	for _, c := range body {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return strings.ToLower(s)
}

// px parses an integer in [min,max] and renders it as "Npx"; ok is false when the
// value is blank or out of range.
func px(s string, min, max int) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min || n > max {
		return "", false
	}
	return strconv.Itoa(n) + "px", true
}
