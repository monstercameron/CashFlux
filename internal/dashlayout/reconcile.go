// SPDX-License-Identifier: MIT

package dashlayout

import "strings"

// IsCustomID reports whether an item id is a user-built (namespaced) widget rather
// than a built-in. Built-in widget ids are bare slugs ("kpi-networth", "recent");
// custom cards published from the Widget Builder are namespaced with a ":" ("wb:my
// chart"). Reconcile preserves custom ids so published cards survive reloads.
func IsCustomID(id string) bool { return strings.Contains(id, ":") }

// Reconcile merges a persisted item list against the current DefaultItems set so
// a saved layout survives across releases: it keeps the user's items, order, and
// sizes, drops any ids that are no longer real widgets, and splices in any
// newly-introduced default widgets near their intended slot — right after the
// nearest earlier default that's already present (so a headline widget lands at
// the top and a footer widget lands at the bottom, instead of all newcomers
// piling up in one place).
func Reconcile(saved []Item) []Item {
	defs := DefaultItems()
	defOrder := make(map[string]int, len(defs))
	for i, d := range defs {
		defOrder[d.ID] = i
	}

	have := make(map[string]bool, len(saved))
	result := make([]Item, 0, len(defs))
	for _, s := range saved {
		_, known := defOrder[s.ID]
		if (known || IsCustomID(s.ID)) && !have[s.ID] {
			result = append(result, s)
			have[s.ID] = true
		}
	}

	for _, d := range defs {
		if have[d.ID] {
			continue
		}
		// Insert after the last already-placed item whose default position precedes
		// this one; if none does, it goes to the front.
		insertAt := 0
		for i, it := range result {
			if doi, ok := defOrder[it.ID]; ok && doi < defOrder[d.ID] {
				insertAt = i + 1
			}
		}
		result = append(result[:insertAt], append([]Item{d}, result[insertAt:]...)...)
		have[d.ID] = true
	}
	return result
}
