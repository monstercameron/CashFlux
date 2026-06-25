// SPDX-License-Identifier: MIT

// Package chartspec is a pure, declarative description of a chart — the typed Go
// side of CashFlux's charting, independent of how it's drawn. A Spec names a
// chart kind, its data series, axes, and display options; helpers validate it and
// compute its data extent (for scaling). A renderer (pure-Go SVG today, possibly
// D3 later) consumes a Spec — the spec itself has no platform dependencies, so
// validation and scale math are verifiable in plain `go test`.
package chartspec

import (
	"errors"
	"fmt"
)

// Kind is the chart type a Spec describes.
type Kind string

// The supported chart kinds.
const (
	Line  Kind = "line"
	Area  Kind = "area"
	Bar   Kind = "bar"
	Donut Kind = "donut"
)

// Valid reports whether k is a known chart kind.
func (k Kind) Valid() bool {
	switch k {
	case Line, Area, Bar, Donut:
		return true
	default:
		return false
	}
}

// Point is one datum: an X/Y pair with an optional label (used for category axes
// and donut slices). JSON-tagged because the spec is serialized to the D3 shim.
type Point struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label,omitempty"`
	// Color is an optional per-datum hex override. The donut renderer already
	// honors it per slice; the bar renderer honors it per bar (empty falls back
	// to the series color). Used to give a ranked bar chart the same categorical
	// palette as its sibling donut so the two read as one picture.
	Color string `json:"color,omitempty"`
}

// Series is a named, colored set of points (one line/area/bar group, or the
// single ring of a donut).
type Series struct {
	Name   string  `json:"name,omitempty"`
	Color  string  `json:"color,omitempty"` // hex; empty lets the renderer pick a default
	Points []Point `json:"points"`
}

// Axis describes one axis: a display label and an optional value format hint
// (e.g. "$,.0f" or "%") the renderer may honor.
type Axis struct {
	Label  string `json:"label,omitempty"`
	Format string `json:"format,omitempty"`
}

// Spec is a complete, declarative chart description.
type Spec struct {
	Kind    Kind     `json:"kind"`
	Series  []Series `json:"series"`
	X       Axis     `json:"x"`
	Y       Axis     `json:"y"`
	Stacked bool     `json:"stacked,omitempty"` // stack multiple series (area/bar)
	Legend  bool     `json:"legend,omitempty"`  // show a legend
}

// Validation errors, exported so callers can branch on them with errors.Is.
var (
	ErrUnknownKind = errors.New("chartspec: unknown chart kind")
	ErrNoSeries    = errors.New("chartspec: a chart needs at least one series")
	ErrEmptySeries = errors.New("chartspec: every series needs at least one point")
	ErrDonutSingle = errors.New("chartspec: a donut chart takes exactly one series")
)

// Validate reports the first problem that would stop the spec from rendering: an
// unknown kind, no series, an empty series, or a multi-series donut. A nil error
// means the spec is drawable.
func (s Spec) Validate() error {
	if !s.Kind.Valid() {
		return fmt.Errorf("%w: %q", ErrUnknownKind, s.Kind)
	}
	if len(s.Series) == 0 {
		return ErrNoSeries
	}
	if s.Kind == Donut && len(s.Series) != 1 {
		return ErrDonutSingle
	}
	for i, ser := range s.Series {
		if len(ser.Points) == 0 {
			return fmt.Errorf("%w: series %d (%q)", ErrEmptySeries, i, ser.Name)
		}
	}
	return nil
}

// Extent returns the min/max of X and Y across every point in every series — the
// data bounds a renderer scales into pixel space. It reports ok=false when there
// are no points (the bounds are then all zero), so callers don't divide by a
// zero-width range.
func (s Spec) Extent() (minX, maxX, minY, maxY float64, ok bool) {
	for _, ser := range s.Series {
		for _, p := range ser.Points {
			if !ok {
				minX, maxX, minY, maxY = p.X, p.X, p.Y, p.Y
				ok = true
				continue
			}
			if p.X < minX {
				minX = p.X
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	return minX, maxX, minY, maxY, ok
}
