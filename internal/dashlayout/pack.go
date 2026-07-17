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
		{ID: "monthly-recap", ColSpan: 4, RowSpan: 1},
		{ID: "kpi-networth", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-income", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-spending", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-liabilities", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-assets", ColSpan: 1, RowSpan: 1},
		{ID: "kpi-safetospend", ColSpan: 1, RowSpan: 1},
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
		{ID: "smart-digest", ColSpan: 2, RowSpan: 2},
		// R25: always-on SMART anomaly hub — four detector types (balance,
		// duplicates, spending spikes, missing transactions) without any opt-in gate.
		{ID: "anomaly-hub", ColSpan: 2, RowSpan: 1},
		// spotlight: a compound widget built entirely from a custom intra-tile content
		// layout (icon + heading + side-by-side figures + divider + templated caption),
		// rendered by the content-layout engine — demonstrates fully custom widgets.
		{ID: "spotlight", ColSpan: 2, RowSpan: 2},
		// goal-states: a compact current / missed / completed goals summary. Appended last
		// in the catalog so it never shifts the arrangement above it; the curated default
		// layout places its own (full-width) copy up in tier 2.
		{ID: "goal-states", ColSpan: 2, RowSpan: 1},
		// forecast: projected available cash at 30/60/90 days from each cash
		// account's recurring-driver projection. Appended last for the same reason.
		{ID: "forecast", ColSpan: 2, RowSpan: 1},
	}
}

// DefaultLayoutItems is the curated *initial* dashboard a fresh install shows —
// a deliberately edited, tiered subset of the full catalog (DefaultItems). It
// excludes tiles whose figures the hero already shows (net worth, income,
// spending, savings rate) and redundant insight tiles (smart-digest duplicates
// the anomaly hub + smart strip; spotlight is a demo triplicate of the hero
// stats), so no number appears twice above the fold. The excluded widgets stay
// in the catalog and can be re-added; only the seed layout is curated.
//
// Ordering follows three tiers: (1) what needs me — attention; (2) snapshot —
// the figures NOT in the hero (assets, liabilities, safe to spend, health);
// (3) how I'm doing — trends, budgets, cash flow, accounts, recent, breakdown,
// goals, to-do, and the low-urgency notices. Spans differ from the catalog
// defaults where the curated layout reads better: trend and budgets are widened
// to two columns so their chart/bars are legible, and accounts gets a second row
// so its balances never clip.
func DefaultLayoutItems() []Item {
	// Importance descends with position so the auto-importance layout mode reproduces
	// this curated priority (the field is otherwise 0 and that mode would fall back to
	// the unrelated catalog order). Spans are chosen so the tiles tile the 4-column
	// grid with NO empty cells: each row's spans sum to 4 (see TestDefaultLayoutPacksGapFree).
	return []Item{
		// Tier 1 — what needs me.
		{ID: "attention", ColSpan: 4, RowSpan: 1, Importance: 100},
		// The month in review: a full-width recap banner right under the attention
		// digest, the "here's how your month went" headline (CG-S1).
		{ID: "monthly-recap", ColSpan: 4, RowSpan: 1, Importance: 96},
		// Tier 2 — snapshot figures the hero does not already show. assets(1) +
		// liabilities(1) + safe-to-spend(2) fills the row exactly; safe-to-spend takes
		// the double width because it carries the longest sub-label.
		{ID: "kpi-assets", ColSpan: 1, RowSpan: 1, Importance: 92},
		{ID: "kpi-liabilities", ColSpan: 1, RowSpan: 1, Importance: 91},
		{ID: "kpi-safetospend", ColSpan: 2, RowSpan: 1, Importance: 90},
		// Goals at a glance: a full-width current / missed / completed strip. Full width
		// (not the catalog's compact 2-wide) so the three counts breathe as a banner, and
		// so it lands as a complete row right after the full KPI row above — keeping the
		// default layout gap-free (see TestDefaultLayoutPacksGapFree).
		{ID: "goal-states", ColSpan: 4, RowSpan: 1, Importance: 88},
		// Tier 3 — how I'm doing.
		{ID: "health", ColSpan: 2, RowSpan: 1, Importance: 82},
		{ID: "trend", ColSpan: 2, RowSpan: 2, Importance: 80},
		{ID: "budgets", ColSpan: 2, RowSpan: 2, Importance: 78},
		{ID: "cashflow", ColSpan: 2, RowSpan: 1, Importance: 72},
		{ID: "bills", ColSpan: 2, RowSpan: 1, Importance: 70},
		{ID: "accounts", ColSpan: 2, RowSpan: 2, Importance: 64},
		{ID: "recent", ColSpan: 2, RowSpan: 2, Importance: 62},
		{ID: "breakdown", ColSpan: 2, RowSpan: 1, Importance: 54},
		// Full-width so the row sums to 4 on its own (gap-free invariant): the
		// forward view (projected cash at 30/60/90 days) as a banner right after
		// the backward view (where the month's money went).
		{ID: "forecast", ColSpan: 4, RowSpan: 1, Importance: 53},
		// Goals and to-do sit two-wide as a paired row so their content has room: the
		// goal's "(due …)" title no longer wraps below its siblings, and to-do task
		// titles read in full instead of truncating mid-word.
		{ID: "goals", ColSpan: 2, RowSpan: 1, Importance: 52},
		{ID: "todo", ColSpan: 2, RowSpan: 1, Importance: 50},
		{ID: "highlight", ColSpan: 2, RowSpan: 1, Importance: 40},
		{ID: "freshness", ColSpan: 2, RowSpan: 1, Importance: 38},
		// anomaly-hub ("Flagged activity") is intentionally NOT in the default view: it
		// duplicates the always-present Smart strip below the bento and otherwise sits as
		// a sparse "no anomalies" card. It stays in the catalog and can be added back.
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
