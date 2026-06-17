package dashlayout

import "sort"

// This file adds the auto-layout engine (TODOS §C24): Arrange reorders the tile
// sequence by a chosen Mode, and the existing Pack then derives positions. Sizes
// are never changed here — tile spans stay user-set (resize handles); auto-layout
// only decides order, so it composes with manual resizing.

// Mode is how the dashboard decides tile order before packing.
type Mode string

const (
	// ModeCustom keeps the user's hand-arranged order (drag-and-drop). Arrange
	// is a no-op for it.
	ModeCustom Mode = "custom"
	// ModeAutoDefault sorts tiles into the canonical built-in order (the order
	// of DefaultItems), ignoring the user's drag history.
	ModeAutoDefault Mode = "auto-default"
	// ModeAutoImportance sorts tiles by their per-tile Importance (higher first),
	// breaking ties with the canonical default order so the result is stable.
	ModeAutoImportance Mode = "auto-importance"
)

// Valid reports whether m is a known layout mode.
func (m Mode) Valid() bool {
	switch m {
	case ModeCustom, ModeAutoDefault, ModeAutoImportance:
		return true
	default:
		return false
	}
}

// canonicalOrder maps each default widget id to its position in DefaultItems,
// used as the auto-default sort key and the importance-mode tiebreak.
func canonicalOrder() map[string]int {
	def := DefaultItems()
	m := make(map[string]int, len(def))
	for i, it := range def {
		m[it.ID] = i
	}
	return m
}

// canonRank returns id's canonical position, or a value past the end for ids not
// in the default set (so unknown/custom tiles sort after the known ones while
// keeping their relative order under a stable sort).
func canonRank(order map[string]int, id string) int {
	if r, ok := order[id]; ok {
		return r
	}
	return len(order)
}

// Arrange returns a reordered copy of items for the given mode, without changing
// any tile's spans. ModeCustom (and any unknown mode) returns the items in their
// existing order. The input is not modified. Deterministic: same items + mode →
// same order. Re-Pack the result to derive grid positions.
func Arrange(items []Item, mode Mode) []Item {
	out := append([]Item(nil), items...)
	switch mode {
	case ModeAutoDefault:
		order := canonicalOrder()
		sort.SliceStable(out, func(i, j int) bool {
			return canonRank(order, out[i].ID) < canonRank(order, out[j].ID)
		})
	case ModeAutoImportance:
		order := canonicalOrder()
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Importance != out[j].Importance {
				return out[i].Importance > out[j].Importance
			}
			return canonRank(order, out[i].ID) < canonRank(order, out[j].ID)
		})
	}
	return out
}
