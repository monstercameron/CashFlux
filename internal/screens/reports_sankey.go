// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// This file draws the Annual Review's money-flow diagram as an in-house SVG
// sankey: smooth cubic-bezier ribbons on the pure reports.LayoutSankey
// geometry, full container width, per-ribbon source→target gradients, and
// labels haloed straight onto the diagram. It replaced the mermaid sankey,
// which capped its own width and drew hair-thin ribbons in a fixed palette.
//
// The SVG is assembled as a string and injected via the managed-container
// pattern (like ui.Mermaid): the framework's SVG-namespace tag set doesn't
// include <text>/<title>, so vdom-built labels land in the HTML namespace and
// never paint. innerHTML parses the whole <svg> as foreign content, which
// namespaces everything correctly. All user-derived strings are escaped.

// rptaFlowColors assigns stable colors to money-flow node labels: spending
// categories rotate through a category palette, income sources through a
// cooler source palette, and the three special nodes (income hub, savings,
// everything-else) get their reserved hues. Greens are reserved for Savings so
// "green = money kept" stays meaningful across the whole report.
type rptaFlowColors struct {
	byLabel map[string]string
	catIdx  int
	srcIdx  int
	accent  string
}

var rptaCatPalette = []string{
	"#d96a55", "#c98a2e", "#b04f74", "#7d6ac2", "#4f86c6",
	"#c26b9b", "#946b48", "#5b8a8f", "#a35a5a", "#8a7a3f",
}

var rptaSrcPalette = []string{
	"#5b9aa9", "#6b87b8", "#9a8fb8", "#b8935b", "#8fa06b",
}

// The report's semantic tones — resolved hex twins of the theme's --up/--warn/
// --down fallbacks, for SVG strokes and chart props that can't take a CSS var.
const (
	rptaToneUp   = "#4ea777"
	rptaToneWarn = "#d8a24a"
	rptaToneDown = "#d8716f"
)

func newRptaFlowColors(accent string) *rptaFlowColors {
	return &rptaFlowColors{byLabel: map[string]string{}, accent: accent}
}

func (c *rptaFlowColors) hub(label string)     { c.byLabel[label] = c.accent }
func (c *rptaFlowColors) savings(label string) { c.byLabel[label] = "#4ea777" }
func (c *rptaFlowColors) rest(label string)    { c.byLabel[label] = "#8a8f98" }

// deficit tones the "Drawn from savings" inflow that appears when the year
// overspent — the app's negative tone, so the gap ribbon reads as a warning.
func (c *rptaFlowColors) deficit(label string) { c.byLabel[label] = "#d8716f" }

func (c *rptaFlowColors) category(label string) {
	c.byLabel[label] = rptaCatPalette[c.catIdx%len(rptaCatPalette)]
	c.catIdx++
}

func (c *rptaFlowColors) source(label string) {
	c.byLabel[label] = rptaSrcPalette[c.srcIdx%len(rptaSrcPalette)]
	c.srcIdx++
}

func (c *rptaFlowColors) of(label string) string {
	if col, ok := c.byLabel[label]; ok {
		return col
	}
	return "#8a8f98"
}

// rptaChartLegend is the one-line key under a report chart: a color swatch
// plus what the plotted series actually is (with its unit).
func rptaChartLegend(color, text string) ui.Node {
	return Div(css.Class("rpta-chart-legend"),
		Span(css.Class("rpta-flow-dot"), Style(map[string]string{"background": color})),
		Span(text),
	)
}

// rptaShortMoney renders minor units as a compact whole-currency figure for
// in-diagram labels ("$6,299") — full-precision amounts live in the tooltips
// and the per-$100 table.
func rptaShortMoney(minor int64, factor int64, symbol string) string {
	v := (minor + factor/2) / factor
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatInt(v, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + symbol + s
	}
	return symbol + s
}

// rptaMoneyFlowSVG renders flows as the full-width smooth-ribbon sankey.
// incomeTotal drives the "% of income" line in the tooltips; fmtMinor formats
// the tooltip amounts; factor/symbol drive the short in-diagram labels.
func rptaMoneyFlowSVG(flows []reports.Flow, colors *rptaFlowColors, incomeTotal int64, fmtMinor func(int64) string, factor int64, symbol string) ui.Node {
	const w, h, nodeW, gap, minH = 1000.0, 430.0, 14.0, 12.0, 4.0
	layout := reports.LayoutSankey(flows, w, h, nodeW, gap, minH)
	if len(layout.Nodes) == 0 {
		return Fragment()
	}

	esc := html.EscapeString
	pctOfIncome := func(v int64) string {
		if incomeTotal <= 0 {
			return ""
		}
		tenths := v * 1000 / incomeTotal
		return fmt.Sprintf(" · %d.%d%% %s", tenths/10, tenths%10, uistate.T("rpta.ofIncome"))
	}

	var b strings.Builder
	// The HOST div (role=img + aria-label, see the render func) owns the
	// accessible description; a second role=img on the inner SVG made assistive
	// tech announce the same description twice, nested (QA CF-30).
	fmt.Fprintf(&b, `<svg class="rpta-flow-svg" viewBox="0 0 %.0f %.0f" aria-hidden="true" data-testid="rpta-flow-svg">`,
		w, h)

	b.WriteString("<defs>")
	for i, l := range layout.Links {
		src, dst := layout.Nodes[l.From], layout.Nodes[l.To]
		fmt.Fprintf(&b, `<linearGradient id="rptaflow-g%d" x1="0" y1="0" x2="1" y2="0"><stop offset="0%%" stop-color="%s"/><stop offset="100%%" stop-color="%s"/></linearGradient>`,
			i, colors.of(src.Label), colors.of(dst.Label))
	}
	b.WriteString("</defs><g>")

	for i, l := range layout.Links {
		src, dst := layout.Nodes[l.From], layout.Nodes[l.To]
		sx, tx := src.X+nodeW, dst.X
		mx := (sx + tx) / 2
		fmt.Fprintf(&b, `<path class="rpta-flow-link" fill="url(#rptaflow-g%d)" d="M %.1f,%.1f C %.1f,%.1f %.1f,%.1f %.1f,%.1f L %.1f,%.1f C %.1f,%.1f %.1f,%.1f %.1f,%.1f Z"><title>%s</title></path>`,
			i,
			sx, l.SY, mx, l.SY, mx, l.TY, tx, l.TY,
			tx, l.TY+l.H, mx, l.TY+l.H, mx, l.SY+l.H, sx, l.SY+l.H,
			esc(src.Label+" → "+dst.Label+" — "+fmtMinor(l.Value)+pctOfIncome(l.Value)))
	}
	b.WriteString("</g><g>")

	for _, n := range layout.Nodes {
		fmt.Fprintf(&b, `<rect class="rpta-flow-node" x="%.1f" y="%.1f" width="%.0f" height="%.1f" rx="3" fill="%s"><title>%s</title></rect>`,
			n.X, n.Y, nodeW, n.H, colors.of(n.Label),
			esc(n.Label+" — "+fmtMinor(n.Value)+pctOfIncome(n.Value)))
	}
	b.WriteString("</g><g>")

	for _, n := range layout.Nodes {
		cy := n.Y + n.H/2 + 4.5
		short := rptaShortMoney(n.Value, factor, symbol)
		label := `<tspan class="rpta-flow-name">` + esc(n.Label) + `</tspan><tspan class="rpta-flow-amt">` + " " + esc(short) + `</tspan>`
		switch n.Col {
		case 0:
			fmt.Fprintf(&b, `<text class="rpta-flow-label" x="%.1f" y="%.1f">%s</text>`, n.X+nodeW+9, cy, label)
		case 1:
			fmt.Fprintf(&b, `<text class="rpta-flow-label" x="%.1f" y="%.1f" text-anchor="middle">%s</text>`, n.X+nodeW/2, cy, label)
		default:
			fmt.Fprintf(&b, `<text class="rpta-flow-label" x="%.1f" y="%.1f" text-anchor="end">%s</text>`, n.X-9, cy, label)
		}
	}
	b.WriteString("</g></svg>")

	return ui.CreateElement(rptaFlowView, rptaFlowProps{SVG: b.String(), Alt: uistate.T("rpta.flowAlt")})
}

// rptaFlowProps carries the pre-built sankey SVG markup into the managed
// container component.
type rptaFlowProps struct {
	SVG string
	Alt string
}

// rptaFlowView owns a container div the effect fills with the sankey SVG —
// the same ref/portal pattern as ui.Mermaid, so the vdom never diffs the SVG
// internals and the browser parses the markup with correct namespaces.
func rptaFlowView(props rptaFlowProps) ui.Node {
	id := ui.UseId()
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil
		}
		el := doc.Call("getElementById", id)
		if !el.Truthy() {
			return nil
		}
		el.Set("innerHTML", props.SVG)
		return func() {
			if el.Truthy() {
				el.Set("innerHTML", "")
			}
		}
	}, props.SVG)
	return Div(Attr("id", id), css.Class("rpta-flow-host"), Attr("role", "img"), Attr("aria-label", props.Alt))
}
