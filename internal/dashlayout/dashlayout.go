// SPDX-License-Identifier: MIT

// Package dashlayout models the dashboard bento layout: where each widget sits
// in the grid (column/row and spans) and the operations the reconfigure UI needs
// — swapping two widgets and resizing spans. Pure Go, no platform or rendering
// dependencies, unit-tested on native Go; the view reads placements to emit CSS
// grid styles and writes back swaps/resizes.
package dashlayout

import (
	"strconv"
)

// Placement is one widget's position in the bento grid (1-based column/row with
// spans). Spans below 1 are treated as 1.
type Placement struct {
	ID      string
	Col     int
	Row     int
	ColSpan int
	RowSpan int
}

// GridColumn renders the CSS grid-column value, e.g. "1" or "1 / span 2".
func (p Placement) GridColumn() string { return axis(p.Col, p.ColSpan) }

// GridRow renders the CSS grid-row value, e.g. "2" or "3 / span 2".
func (p Placement) GridRow() string { return axis(p.Row, p.RowSpan) }

func axis(start, span int) string {
	if span > 1 {
		return strconv.Itoa(start) + " / span " + strconv.Itoa(span)
	}
	return strconv.Itoa(start)
}

// Layout is the ordered set of widget placements.
type Layout []Placement

// Default is the candidate-C dashboard arrangement (the header cell at row 1 is
// fixed and not part of the layout).
func Default() Layout {
	return Layout{
		{ID: "kpi-networth", Col: 1, Row: 2, ColSpan: 1, RowSpan: 1},
		{ID: "kpi-income", Col: 2, Row: 2, ColSpan: 1, RowSpan: 1},
		{ID: "kpi-spending", Col: 3, Row: 2, ColSpan: 1, RowSpan: 1},
		{ID: "kpi-liabilities", Col: 4, Row: 2, ColSpan: 1, RowSpan: 1},
		{ID: "recent", Col: 1, Row: 3, ColSpan: 2, RowSpan: 2},
		{ID: "budgets", Col: 3, Row: 3, ColSpan: 1, RowSpan: 2},
		{ID: "trend", Col: 4, Row: 3, ColSpan: 1, RowSpan: 2},
		{ID: "goals", Col: 1, Row: 5, ColSpan: 1, RowSpan: 1},
		{ID: "todo", Col: 2, Row: 5, ColSpan: 1, RowSpan: 1},
		{ID: "accounts", Col: 3, Row: 5, ColSpan: 2, RowSpan: 1},
		{ID: "cashflow", Col: 1, Row: 6, ColSpan: 2, RowSpan: 1},
		{ID: "bills", Col: 3, Row: 6, ColSpan: 2, RowSpan: 1},
		{ID: "savings", Col: 1, Row: 7, ColSpan: 2, RowSpan: 1},
		{ID: "breakdown", Col: 3, Row: 7, ColSpan: 2, RowSpan: 1},
		{ID: "freshness", Col: 1, Row: 8, ColSpan: 4, RowSpan: 1},
	}
}

// indexOf returns the slice index of the placement with the given id, or -1.
func (l Layout) indexOf(id string) int {
	for i := range l {
		if l[i].ID == id {
			return i
		}
	}
	return -1
}

// Get returns the placement for id and whether it was found.
func (l Layout) Get(id string) (Placement, bool) {
	if i := l.indexOf(id); i >= 0 {
		return l[i], true
	}
	return Placement{}, false
}

// Swap returns a copy of the layout with the grid positions (column, row, and
// both spans) of the two widgets exchanged, keeping their IDs. Unknown ids leave
// the layout unchanged.
func (l Layout) Swap(idA, idB string) Layout {
	out := append(Layout(nil), l...)
	ia, ib := out.indexOf(idA), out.indexOf(idB)
	if ia < 0 || ib < 0 || ia == ib {
		return out
	}
	out[ia].Col, out[ib].Col = out[ib].Col, out[ia].Col
	out[ia].Row, out[ib].Row = out[ib].Row, out[ia].Row
	out[ia].ColSpan, out[ib].ColSpan = out[ib].ColSpan, out[ia].ColSpan
	out[ia].RowSpan, out[ib].RowSpan = out[ib].RowSpan, out[ia].RowSpan
	return out
}

// Resize returns a copy of the layout with the given widget's spans set, clamped
// to a minimum of 1. Unknown ids leave the layout unchanged.
func (l Layout) Resize(id string, colSpan, rowSpan int) Layout {
	out := append(Layout(nil), l...)
	i := out.indexOf(id)
	if i < 0 {
		return out
	}
	if colSpan < 1 {
		colSpan = 1
	}
	if rowSpan < 1 {
		rowSpan = 1
	}
	out[i].ColSpan = colSpan
	out[i].RowSpan = rowSpan
	return out
}
