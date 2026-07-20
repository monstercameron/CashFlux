// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/chart"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// AreaChartProps configures an AreaChart.
type AreaChartProps struct {
	Values     []float64
	Stroke     string  // line + gradient color (hex), default candidate-C up green
	GradientID string  // unique gradient id when several charts share a page; default "cf-area"
	Width      float64 // viewBox width, default 180
	Height     float64 // viewBox height, default 90
	Label      string  // accessible description (the SVG is role="img"); default "Trend chart"
	// Labels are optional x-axis period captions (e.g. month names) rendered as an
	// HTML row beneath the chart. They live outside the SVG because the chart uses
	// preserveAspectRatio="none" (non-uniform scaling) which would distort SVG text.
	Labels []string
	// ValueLabels are optional pre-formatted per-point values (e.g. "$1,480.00", "17%"),
	// parallel to Values. When supplied, an invisible hover target is drawn at each point
	// with a <title> of "<period>: <value>" so the trend's exact value is readable on hover
	// (the caller formats so units stay correct — money vs percent). No visible change.
	ValueLabels []string
}

// AreaChart renders a filled area sparkline from a value series using the pure
// internal/chart geometry: a soft top-to-bottom gradient fill under a stroked
// line. Stretches to its container width (non-uniform scaling, like the mockup).
//
// AXIS POLICY (UI/UX task #33): charts split into two classes and each keeps
// one treatment. TREND-class charts (this component — hero/net-worth/debt
// sparklines) are axis-free: HTML period captions below (Labels) and exact
// values on hover (ValueLabels) — pass BOTH so a trend is never unreadable.
// OPERATIONAL charts (the planning cash-runway, cash-flow bars) carry labeled
// axes/gridlines, because users act on their absolute values. Don't add SVG
// axis text here, and don't strip the axes there.
func AreaChart(props AreaChartProps) uic.Node {
	w, h := props.Width, props.Height
	if w == 0 {
		w = 180
	}
	if h == 0 {
		h = 90
	}
	stroke := props.Stroke
	if stroke == "" {
		stroke = "#2e8b57"
	}
	gid := props.GradientID
	if gid == "" {
		gid = "cf-area"
	}
	label := props.Label
	if label == "" {
		label = "Trend chart"
	}

	pts := chart.Points(props.Values, w, h, 6)
	area := chart.AreaPath(pts, h)
	line := chart.LinePath(pts)

	svgArgs := []any{
		css.Class(tw.WFull, tw.MtAuto),
		Attr("role", "img"),
		Attr("aria-label", label),
		Attr("viewBox", fmt.Sprintf("0 0 %g %g", w, h)),
		Attr("preserveAspectRatio", "none"),
		Attr("height", "120"),
		Tag("defs",
			Tag("linearGradient",
				Attr("id", gid), Attr("x1", "0"), Attr("y1", "0"), Attr("x2", "0"), Attr("y2", "1"),
				Tag("stop", Attr("offset", "0"), Attr("stop-color", stroke), Attr("stop-opacity", ".25")),
				Tag("stop", Attr("offset", "1"), Attr("stop-color", stroke), Attr("stop-opacity", "0")),
			),
		),
		Path(Attr("d", area), Attr("fill", "url(#"+gid+")"), css.Class("wonder-chart-area")),
		Path(Attr("d", line), Attr("fill", "none"), Attr("stroke", stroke), Attr("stroke-width", "2"), Attr("pathLength", "1"), css.Class("wonder-chart-line")),
	}
	// Optional invisible per-point hover targets (transparent so there's no visible change; under
	// preserveAspectRatio="none" they stretch into ellipses but still receive the pointer + show the
	// <title>). Each reads "<period>: <value>" so the trend's exact value is identifiable on hover.
	if n := len(pts); n > 0 && len(props.ValueLabels) == n {
		for i, p := range pts {
			tip := props.ValueLabels[i]
			if i < len(props.Labels) && props.Labels[i] != "" {
				tip = props.Labels[i] + ": " + tip
			}
			svgArgs = append(svgArgs, Tag("circle",
				Attr("cx", fmt.Sprintf("%g", p.X)), Attr("cy", fmt.Sprintf("%g", p.Y)),
				Attr("r", "7"), Attr("fill", "transparent"),
				Tag("title", Text(tip)),
			))
		}
	}
	svg := Svg(svgArgs...)
	if len(props.Labels) == 0 {
		return svg
	}
	// R-4: render calendar/period captions as an evenly-spaced HTML row under the chart.
	spans := make([]any, 0, len(props.Labels)+1)
	spans = append(spans, css.Class("area-labels", tw.Flex, tw.JustifyBetween, tw.Text11, tw.TextFaint, tw.Mt1))
	for _, lbl := range props.Labels {
		spans = append(spans, Span(lbl))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol), svg, Div(spans...))
}
