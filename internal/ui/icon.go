//go:build js && wasm

// Package ui holds reusable, props-driven view primitives shared across every
// CashFlux screen — the Go port of the candidate-C design system
// (design/candidate-c.html). Components here are generic building blocks (icons,
// the widget shell, the flip panel, control primitives); screens compose them
// rather than re-authoring bespoke markup. All business logic stays in the pure
// internal/* packages; these primitives only render.
package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Icon renders one stroked line icon from the curated icon set as an inline SVG
// that inherits its color from `currentColor` and its size from the caller's
// classes (e.g. Class("w-4 h-4 shrink-0")). The name is a compile-checked
// icon.Name, so call sites can't reference an icon that doesn't exist. Extra prop
// options (classes, inline styles) are applied after the shared viewBox/stroke
// defaults so callers can size and tint each icon at its site.
func Icon(name icon.Name, extra ...PropOption) ui.Node {
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

// iconBody returns the SVG child shapes for an icon, matching the mockup. The
// canonical vocabulary lives in internal/icon; this maps each name to its shapes.
func iconBody(name icon.Name) []any {
	switch name {
	case icon.Dashboard:
		return []any{
			Rect(Attr("x", "3"), Attr("y", "3"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
			Rect(Attr("x", "14"), Attr("y", "3"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
			Rect(Attr("x", "14"), Attr("y", "14"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
			Rect(Attr("x", "3"), Attr("y", "14"), Attr("width", "7"), Attr("height", "7"), Attr("rx", "1")),
		}
	case icon.Accounts:
		return []any{
			Rect(Attr("x", "3"), Attr("y", "6"), Attr("width", "18"), Attr("height", "13"), Attr("rx", "2")),
			Path(Attr("d", "M3 10h18")),
			Circle(Attr("cx", "16.5"), Attr("cy", "14.5"), Attr("r", "1.1")),
		}
	case icon.Transactions:
		return []any{
			Path(Attr("d", "M16 3l4 4-4 4")),
			Path(Attr("d", "M20 7H5")),
			Path(Attr("d", "M8 21l-4-4 4-4")),
			Path(Attr("d", "M4 17h15")),
		}
	case icon.Budgets:
		return []any{
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "9")),
			Path(Attr("d", "M12 3a9 9 0 0 1 9 9h-9z")),
		}
	case icon.Goals:
		return []any{
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "9")),
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "5")),
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "1.2")),
		}
	case icon.Todo:
		return []any{
			Rect(Attr("x", "3"), Attr("y", "3"), Attr("width", "18"), Attr("height", "18"), Attr("rx", "2")),
			Path(Attr("d", "M8 12l3 3 5-6")),
		}
	case icon.Settings:
		return []any{
			Path(Attr("d", "M20 7h-9")),
			Path(Attr("d", "M14 17H5")),
			Circle(Attr("cx", "17"), Attr("cy", "17"), Attr("r", "3")),
			Circle(Attr("cx", "7"), Attr("cy", "7"), Attr("r", "3")),
		}
	case icon.Page:
		return []any{
			Path(Attr("d", "M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z")),
			Path(Attr("d", "M14 3v5h5")),
		}
	case icon.Plus:
		return []any{
			Path(Attr("d", "M12 5v14M5 12h14")),
		}
	case icon.Menu:
		return []any{
			Rect(Attr("x", "3"), Attr("y", "4"), Attr("width", "18"), Attr("height", "16"), Attr("rx", "2")),
			Path(Attr("d", "M9 4v16")),
		}
	case icon.Tag:
		return []any{
			Path(Attr("d", "M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z")),
			Circle(Attr("cx", "7"), Attr("cy", "7"), Attr("r", "1.4")),
		}
	case icon.Users:
		return []any{
			Circle(Attr("cx", "9"), Attr("cy", "8"), Attr("r", "3")),
			Path(Attr("d", "M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6")),
			Path(Attr("d", "M16 5.3a3 3 0 0 1 0 5.4")),
			Path(Attr("d", "M21 20c0-2.6-1.6-4.8-3.9-5.7")),
		}
	case icon.Planning:
		return []any{
			Path(Attr("d", "M4 19V5")),
			Path(Attr("d", "M4 19h16")),
			Path(Attr("d", "M7 15l3-4 3 2 4-6")),
		}
	case icon.Allocate:
		return []any{
			Circle(Attr("cx", "12"), Attr("cy", "12"), Attr("r", "9")),
			Path(Attr("d", "M12 3v9l7 4")),
		}
	case icon.Insights:
		return []any{
			Path(Attr("d", "M9 18h6")),
			Path(Attr("d", "M10 21h4")),
			Path(Attr("d", "M12 3a6 6 0 0 1 4 10.5c-.7.7-1 1.2-1 2.5H9c0-1.3-.3-1.8-1-2.5A6 6 0 0 1 12 3z")),
		}
	case icon.Customize:
		return []any{
			Path(Attr("d", "M4 7h16")),
			Path(Attr("d", "M4 12h16")),
			Path(Attr("d", "M4 17h16")),
			Circle(Attr("cx", "9"), Attr("cy", "7"), Attr("r", "1.8")),
			Circle(Attr("cx", "15"), Attr("cy", "12"), Attr("r", "1.8")),
			Circle(Attr("cx", "7"), Attr("cy", "17"), Attr("r", "1.8")),
		}
	default:
		return nil
	}
}
