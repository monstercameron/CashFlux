package dashlayout

// Reconcile merges a persisted item list against the current DefaultItems set so
// a saved layout survives across releases: it keeps the user's items and order,
// drops any ids that are no longer real widgets, and surfaces newly-introduced
// default widgets at the top (in default order) rather than hiding them at the
// bottom. Spans on kept items are preserved exactly as the user arranged them.
func Reconcile(saved []Item) []Item {
	defs := DefaultItems()
	known := make(map[string]bool, len(defs))
	for _, d := range defs {
		known[d.ID] = true
	}

	have := make(map[string]bool, len(saved))
	kept := make([]Item, 0, len(saved))
	for _, s := range saved {
		if known[s.ID] && !have[s.ID] {
			kept = append(kept, s)
			have[s.ID] = true
		}
	}

	var missing []Item
	for _, d := range defs {
		if !have[d.ID] {
			missing = append(missing, d)
		}
	}
	return append(missing, kept...)
}
