// SPDX-License-Identifier: MIT

// Package chart turns a series of values into SVG path data for the dashboard's
// sparkline/area charts. It is pure geometry — no platform or rendering
// dependencies — so it unit-tests on native Go; the view layer (internal/ui)
// feeds the resulting path strings to an <svg>.
package chart

import (
	"strconv"
	"strings"
)

// Point is a coordinate in the SVG user space.
type Point struct {
	X float64
	Y float64
}

// Points maps values to coordinates in a w×h box. X is evenly spaced across the
// width; Y is inverted (SVG y grows downward) and scaled so the series min sits
// near the bottom and the max near the top, leaving `pad` vertical padding. A
// flat series (or a single point) is centered vertically.
func Points(values []float64, w, h, pad float64) []Point {
	n := len(values)
	if n == 0 {
		return nil
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min

	pts := make([]Point, n)
	for i, v := range values {
		x := w / 2
		if n > 1 {
			x = float64(i) / float64(n-1) * w
		}
		y := h / 2
		if span != 0 {
			norm := (v - min) / span // 0 at min, 1 at max
			y = pad + (1-norm)*(h-2*pad)
		}
		pts[i] = Point{X: x, Y: y}
	}
	return pts
}

// LinePath returns the open SVG path ("M x,y L x,y …") through the points.
func LinePath(pts []Point) string {
	if len(pts) == 0 {
		return ""
	}
	var b strings.Builder
	for i, p := range pts {
		if i == 0 {
			b.WriteString("M")
		} else {
			b.WriteString(" L")
		}
		b.WriteString(num(p.X))
		b.WriteString(",")
		b.WriteString(num(p.Y))
	}
	return b.String()
}

// AreaPath returns the LinePath closed down to a horizontal baseline and back,
// suitable for filling under the curve.
func AreaPath(pts []Point, baseline float64) string {
	if len(pts) == 0 {
		return ""
	}
	first, last := pts[0], pts[len(pts)-1]
	return LinePath(pts) +
		" L" + num(last.X) + "," + num(baseline) +
		" L" + num(first.X) + "," + num(baseline) + " Z"
}

// num formats a coordinate with fixed precision for stable, compact output.
func num(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// ValueY returns the y coordinate a value maps to under the same scale Points
// uses, and whether that value falls inside the series' own range.
//
// It exists so a caller can draw a reference line at a meaningful value — zero,
// on a projected-balance chart — without duplicating (and drifting from) the
// scaling above. ok is false when the value sits outside the data's range,
// because a reference line clamped to the edge of a chart is a line that says
// something untrue about where zero is (V-sweep C358).
func ValueY(values []float64, v, h, pad float64) (y float64, ok bool) {
	if len(values) == 0 {
		return 0, false
	}
	min, max := values[0], values[0]
	for _, x := range values {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}
	if v < min || v > max {
		return 0, false
	}
	span := max - min
	if span == 0 {
		return h / 2, true
	}
	norm := (v - min) / span
	return pad + (1-norm)*(h-2*pad), true
}
