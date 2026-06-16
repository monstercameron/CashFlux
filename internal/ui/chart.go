//go:build js && wasm

package ui

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/chart"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// AreaChartProps configures an AreaChart.
type AreaChartProps struct {
	Values     []float64
	Stroke     string  // line + gradient color (hex), default candidate-C up green
	GradientID string  // unique gradient id when several charts share a page; default "cf-area"
	Width      float64 // viewBox width, default 180
	Height     float64 // viewBox height, default 90
	Label      string  // accessible description (the SVG is role="img"); default "Trend chart"
}

// AreaChart renders a filled area sparkline from a value series using the pure
// internal/chart geometry: a soft top-to-bottom gradient fill under a stroked
// line. Stretches to its container width (non-uniform scaling, like the mockup).
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
		stroke = "#54b884"
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

	return Svg(
		Class("w-full mt-auto"),
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
		Path(Attr("d", area), Attr("fill", "url(#"+gid+")")),
		Path(Attr("d", line), Attr("fill", "none"), Attr("stroke", stroke), Attr("stroke-width", "2")),
	)
}
