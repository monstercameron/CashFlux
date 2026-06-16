//go:build js && wasm

// Package ui holds reusable, props-driven view primitives shared across every
// CashFlux screen — the Go port of the candidate-C design system
// (design/candidate-c.html). Components here are generic building blocks (icons,
// the widget shell, the flip panel, control primitives); screens compose them
// rather than re-authoring bespoke markup. All business logic stays in the pure
// internal/* packages; these primitives only render.
package ui

import (
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Icon renders one stroked line icon from the candidate-C set as an inline SVG
// that inherits its color from `currentColor` and its size from the caller's
// classes (e.g. Class("w-4 h-4 shrink-0")). Unknown names render an empty SVG.
// Extra prop options (classes, inline styles) are applied after the shared
// viewBox/stroke defaults so callers can size and tint each icon at its site.
func Icon(name string, extra ...PropOption) ui.Node {
	args := []any{
		Attr("viewBox", "0 0 24 24"),
		Attr("fill", "none"),
		Attr("stroke", "currentColor"),
		Attr("stroke-width", "1.6"),
		Attr("stroke-linecap", "round"),
		Attr("stroke-linejoin", "round"),
	}
	for _, e := range extra {
		args = append(args, e)
	}
	return Svg(append(args, iconBody(name)...)...)
}

// iconBody returns the SVG child shapes for a named icon, matching the mockup.
func iconBody(name string) []any {
	switch name {
	case "dashboard":
		return []any{
			Rect(Attr("x", "3"), Attr("y", "3"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
			Rect(Attr("x", "14"), Attr("y", "3"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
			Rect(Attr("x", "14"), Attr("y", "14"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
			Rect(Attr("x", "3"), Attr("y", "14"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
		}
	case "accounts":
		return []any{
			Rect(Attr("x", "3"), Attr("y", "6"), Attr("width", "18"), Attr("height", "13"), Attr("rx", "2")),
			Path(Attr("d", "M3 10h18")),
			Circle(Attr("cx", "16.5"), Attr("cy", "14.5"), Attr("r", "1.1")),
		}
	case "transactions":
		return []any{
			Path(Attr("d", "M16 3l4 4-4 4")),
			Path(Attr("d", "M20 7H5")),
			Path(Attr("d", "M8 21l-4-4 4-4")),
			Path(Attr("d", "M4 17h15")),
		}
	case "budgets":
		return []any{
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "9")),
			Path(Attr("d", "M12 3a9 9 0 0 1 9 9h-9z")),
		}
	case "goals":
		return []any{
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "9")),
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "5")),
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "1.2")),
		}
	case "todo":
		return []any{
			Rect(Attr("x", "3"), Attr("y", "3"), Attr("width", "18"), Attr("height", "18"), Attr("rx", "2")),
			Path(Attr("d", "M8 12l3 3 5-6")),
		}
	case "settings":
		return []any{
			Path(Attr("d", "M20 7h-9")),
			Path(Attr("d", "M14 17H5")),
			Circle(Attr("cx", "17"), Attr("cy", "17"), Attr("r", "3")),
			Circle(Attr("cx", "7"), Attr("cy", "7"), Attr("r", "3")),
		}
	case "page":
		return []any{
			Path(Attr("d", "M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z")),
			Path(Attr("d", "M14 3v5h5")),
		}
	case "plus":
		return []any{
			Path(Attr("d", "M12 5v14M5 12h14")),
		}
	default:
		return nil
	}
}
