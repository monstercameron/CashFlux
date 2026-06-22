//go:build js && wasm

// Package ui holds reusable, props-driven view primitives shared across every
// CashFlux screen — the Go port of the candidate-C design system
// (design/candidate-c.html). Components here are generic building blocks (icons,
// the widget shell, the flip panel, control primitives); screens compose them
// rather than re-authoring bespoke markup. All business logic stays in the pure
// internal/* packages; these primitives only render.
package ui

import (
	"regexp"

	"github.com/monstercameron/CashFlux/internal/icon"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Icon renders one stroked line icon from the curated icon set as an inline SVG
// that inherits its color from `currentColor` and its size from the caller's
// classes (e.g. css.Class(tw.W4, tw.H4, tw.ShrinkO)). The name is a compile-checked
// icon.Name, so call sites can't reference an icon that doesn't exist. Extra prop
// options (classes, inline styles) are applied after the shared viewBox/stroke
// defaults so callers can size and tint each icon at its site.
func Icon(name icon.Name, extra ...PropOption) ui.Node {
	args := []any{
		Attr("viewBox", "0 0 24 24"),
		Attr("fill", "none"),
		Attr("stroke", "currentColor"),
		// Line weight follows the theme's --icon-stroke token (default 1.6). An
		// inline style is used rather than the stroke-width presentation attribute
		// because SVG attributes don't accept var(), while the CSS property does —
		// and inline style beats the attribute, so themed weight always wins.
		Style(map[string]string{"stroke-width": "var(--icon-stroke, 1.6)"}),
		Attr("stroke-linecap", "round"),
		Attr("stroke-linejoin", "round"),
	}
	for _, e := range extra {
		args = append(args, e)
	}
	return Svg(append(args, iconBody(name)...)...)
}

// iconElemRe matches one self-closing shape element (<path .../>, <circle .../>,
// <rect .../>); iconAttrRe pulls its key="value" attributes. Icon path data never
// contains '>' or '"', so these stay simple and robust.
var (
	iconElemRe = regexp.MustCompile(`<(path|circle|rect)\b([^>]*?)/?>`)
	iconAttrRe = regexp.MustCompile(`([\w:-]+)="([^"]*)"`)
)

// iconBody returns the SVG child shapes for an icon by rendering the canonical
// inner markup from internal/icon — the single source of truth for the whole
// curated set. This means every Name in the package renders (not just a
// hand-maintained subset), so newly added glyphs can't silently show blank. The
// markup is a flat list of stroked path/circle/rect elements; we parse it into the
// equivalent shorthand nodes (no syscall/js, no per-icon hooks).
func iconBody(name icon.Name) []any {
	inner := name.Inner()
	if inner == "" {
		return nil
	}
	var out []any
	for _, el := range iconElemRe.FindAllStringSubmatch(inner, -1) {
		var attrs []any
		for _, a := range iconAttrRe.FindAllStringSubmatch(el[2], -1) {
			attrs = append(attrs, Attr(a[1], a[2]))
		}
		switch el[1] {
		case "path":
			out = append(out, Path(attrs...))
		case "circle":
			out = append(out, Circle(attrs...))
		case "rect":
			out = append(out, Rect(attrs...))
		}
	}
	return out
}
