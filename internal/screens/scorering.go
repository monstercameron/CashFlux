// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// scoreRingNode renders the shared SVG circular gauge used by both the
// financial-health and credit-health screens.
//
// Parameters:
//   - pct       — 0–100 fill amount (the arc length as a percentage).
//   - ringColor — CSS color string for the arc stroke (e.g. "hsl(120, 64%, 52%)").
//   - size      — outer pixel diameter of the widget.
//   - ariaLabel — role=img accessible name; callers supply a localized sentence
//     that includes the score and band so the ring is self-describing to screen
//     readers (R52/R64 a11y). The numeric overlay below is aria-hidden.
//   - centerLabel — the big figure node overlaid at the centre of the ring.
//   - subLabel    — the small caption rendered beneath the figure (e.g. "out of 100").
//
// The SVG geometry is fixed: 120×120 viewBox, r=52, stroke-width=10, cx/cy=60.
// The arc starts at 12 o'clock (rotate −90°), has a rounded linecap, and
// animates its dash-offset and stroke color on mount.
func scoreRingNode(pct float64, ringColor string, size int, ariaLabel string, centerLabel, subLabel ui.Node) ui.Node {
	const radius = 52.0
	const circ = 2 * 3.141592653589793 * radius
	offset := circ * (1 - pct/100)
	px := fmt.Sprintf("%dpx", size)

	ring := Svg(
		Attr("viewBox", "0 0 120 120"),
		Attr("width", px), Attr("height", px),
		Attr("role", "img"), Attr("aria-label", ariaLabel),
		// Faint full track.
		Circle(Attr("cx", "60"), Attr("cy", "60"), Attr("r", "52"),
			Attr("fill", "none"), Attr("stroke", "var(--border)"), Attr("stroke-width", "10")),
		// Score arc — starts at 12 o'clock (rotate -90), rounded cap, animates length.
		Circle(Attr("cx", "60"), Attr("cy", "60"), Attr("r", "52"),
			Attr("fill", "none"), Attr("stroke", ringColor), Attr("stroke-width", "10"),
			Attr("stroke-linecap", "round"),
			Attr("stroke-dasharray", fmt.Sprintf("%.2f", circ)),
			Attr("stroke-dashoffset", fmt.Sprintf("%.2f", offset)),
			Attr("transform", "rotate(-90 60 60)"),
			Style(map[string]string{"transition": "stroke-dashoffset .9s cubic-bezier(.22,1,.36,1), stroke .6s ease"})),
	)

	overlay := Div(
		// Visual duplicate of the score the ring's aria-label already announces.
		Attr("aria-hidden", "true"),
		Style(map[string]string{
			"position": "absolute", "inset": "0",
			"display": "flex", "flex-direction": "column",
			"align-items": "center", "justify-content": "center",
		}),
		centerLabel,
		subLabel,
	)

	return Div(
		Style(map[string]string{"position": "relative", "width": px, "height": px, "flex": "0 0 " + px}),
		ring, overlay,
	)
}
