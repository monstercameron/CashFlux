// SPDX-License-Identifier: MIT

package styles

// registerRecurTasksSurface emits the To-do reminder-chip styling that layers on
// top of the generated .todo-meta-item base: the reminder badge (bell + lead
// label) is tinted with the accent so it reads as a distinct, forward-looking
// nudge next to the neutral recurrence badge. Token-based throughout so it tracks
// every theme; lives in its own file so the recurring-task feature owns its rules
// rather than editing the generated sheet.
func registerRecurTasksSurface() {
	// The reminder chip: an accent-tinted pill so an upcoming nudge stands apart
	// from the quieter recurrence badge on the same meta line.
	rule(".todo-meta-item.is-reminder",
		prop("color", "var(--accent)"),
	)
	rule(".todo-meta-item.is-reminder svg",
		prop("color", "var(--accent)"),
		prop("opacity", "0.9"),
	)
}
