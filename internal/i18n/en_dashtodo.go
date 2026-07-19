// SPDX-License-Identifier: MIT

package i18n

// dashTodoKeys holds English strings added by the 2026-07-19 Dashboard + To-do
// UX polish (shorter hero, primary/secondary widget tone, To-do completion count,
// expandable task notes). Kept in its own map and merged via init so the shared
// en.go is never touched by this concurrent lane (mirrors en_lane4.go).
var dashTodoKeys = Catalog{
	// To-do completion tile: the raw numerator/denominator shown beneath the big
	// completion percentage so the percent is legible at a glance.
	// %d ×2 = tasks done, total tasks.
	"todo.doneCount": "%d of %d done",
	// Hint shown (as a tooltip) on a clamped task note that can be expanded.
	"todo.noteExpandHint": "Hover or focus to read the full note",
}

func init() {
	for k, v := range dashTodoKeys {
		english[k] = v
	}
}
