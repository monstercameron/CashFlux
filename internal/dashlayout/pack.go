// SPDX-License-Identifier: MIT

package dashlayout

// This file holds the ordered-sequence + bin-packing model that drives the
// iOS-home-screen-style dashboard: widgets have an order and an intrinsic size,
// and Pack flows them into a fixed-width grid (no overlap, honoring spans). Drag
// becomes "reorder the sequence, then re-Pack", so the other tiles reflow to
// fill the gap — replacing the old pairwise Swap of absolute positions. See
// TODOS §B2. The legacy Placement/Swap/Resize API in dashlayout.go remains until
// the dashboard UI is migrated onto this model.

// Item is a widget's identity and intrinsic grid size. Its position in the
// ordered slice is its placement priority — earlier items are packed first.
// Importance is the user-set priority used by the auto-importance layout mode
// (higher is more important); it is ignored in custom and auto-default modes and
// defaults to 0, so older persisted layouts (which omit it) keep working.
type Item struct {
	ID         string
	ColSpan    int
	RowSpan    int
	Importance int `json:",omitempty"`
}

// DefaultItems is the default dashboard order and tile sizes. Packing it with
// Pack(DefaultItems(), 4) reproduces the original bento arrangement (with rows
// numbered from 1; the dashboard's fixed header occupies grid row 1, so the view
// offsets packed rows by one).
func DefaultItems() []Item {
	return []Item{
		{ID: "attention", ColSpan: 4, RowSpan: 1},
		{ID: "kpi-networth", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-income", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-spending", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-liabilities", ColSpan: 1, RowSpan: 1},
		{ID: "recent", ColSpan: 2, RowSpan: 2},
		{ID: "budgets", ColSpan: 1, RowSpan: 2},
		{ID: "trend", ColSpan: 1, RowSpan: 2},
		{ID: "goals", ColSpan: 1, RowSpan: 1},
		{ID: "todo", ColSpan: 1, RowSpan: 1},
		{ID: "accounts", ColSpan: 2, RowSpan: 1},
		{ID: "cashflow", ColSpan: 2, RowSpan: 1},
		{ID: "bills", ColSpan: 2, RowSpan: 1},
		{ID: "savings", ColSpan: 2, RowSpan: 1},
		{ID: "health", ColSpan: 2, RowSpan: 1},
		{ID: "breakdown", ColSpan: 2, RowSpan: 1},
		{ID: "freshness", ColSpan: 2, RowSpan: 1},
		{ID: "highlight", ColSpan: 2, RowSpan: 1},
		{ID: "smart-digest", ColSpan: 2, RowSpan: 1},
	}
}

// Pack flows items into a cols-wide grid using first-fit placement (scan
// row-major, top-to-bottom then left-to-right, place each item in the earliest
// slot where its whole span fits without overlapping an already-placed tile),
// and returns the resulting Layout with 1-based Col/Row. Spans are clamped to
// at least 1, and a column span wider than the grid is clamped to cols. The
// input is not modified. Deterministic: same items + cols → same Layout.
func Pack(items []Item, cols int) Layout {
	if cols < 1 {
		cols = 1
	}
	occupied := map[[2]int]bool{}
	fits := func(r, c, cs, rs int) bool {
		for dr := 0; dr < rs; dr++ {
			for dc := 0; dc < cs; dc++ {
				if occupied[[2]int{r + dr, c + dc}] {
					return false
				}
			}
		}
		return true
	}
	out := make(Layout, 0, len(items))
	for _, it := range items {
		cs, rs := it.ColSpan, it.RowSpan
		if cs < 1 {
			cs = 1
		}
		if rs < 1 {
			rs = 1
		}
		if cs > cols {
			cs = cols
		}
		placed := false
		for r := 0; !placed; r++ {
			for c := 0; c+cs <= cols; c++ {
				if !fits(r, c, cs, rs) {
					continue
				}
				for dr := 0; dr < rs; dr++ {
					for dc := 0; dc < cs; dc++ {
						occupied[[2]int{r + dr, c + dc}] = true
					}
				}
				out = append(out, Placement{ID: it.ID, Col: c + 1, Row: r + 1, ColSpan: cs, RowSpan: rs})
				placed = true
				break
			}
		}
	}
	return out
}

// indexOfItem returns the slice index of the item with the given id, or -1.
func indexOfItem(items []Item, id string) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}

// Move returns a copy of items with the item identified by id relocated to
// toIndex (clamped to the valid range), preserving the relative order of the
// others — the reorder a drag-and-drop produces before re-Packing. Unknown ids
// return an unchanged copy. The input is not modified.
func Move(items []Item, id string, toIndex int) []Item {
	from := indexOfItem(items, id)
	out := append([]Item(nil), items...)
	if from < 0 {
		return out
	}
	if toIndex < 0 {
		toIndex = 0
	}
	if toIndex >= len(out) {
		toIndex = len(out) - 1
	}
	if toIndex == from {
		return out
	}
	moved := out[from]
	out = append(out[:from], out[from+1:]...)
	// Insert at toIndex of the shortened slice.
	out = append(out, Item{})
	copy(out[toIndex+1:], out[toIndex:])
	out[toIndex] = moved
	return out
}

// Resize returns a copy of items with the given widget's spans set, clamped to
// a minimum of 1. Unknown ids return an unchanged copy. The input is not
// modified. Re-Pack afterward to reflow the grid around the new size.
func ResizeItem(items []Item, id string, colSpan, rowSpan int) []Item {
	out := append([]Item(nil), items...)
	i := indexOfItem(out, id)
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
