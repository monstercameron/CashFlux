// SPDX-License-Identifier: MIT

package styles

// registerDetail4 carries the CSS for the 2026-07-19 fine-detail polish lane 4:
//   - a neutral "Watch" urgency badge (.pace-planwatch) for the Goals "Needs a plan"
//     grades, sitting below Slipping (amber) and Far behind (red);
//   - the Goals Compare one-sentence verdict banner; and
//   - the To-do task note that clamps to two lines and expands on click.
//
// Theme tokens only, so light and dark track automatically.
func registerDetail4() {
	// Neutral urgency badge — the mildest "Needs a plan" grade. Calm, like .pace-ontrack,
	// so it reads clearly quieter than the amber Slipping and red Far behind grades.
	rule(".pace-planwatch",
		prop("background", "var(--bg-elev)"),
		prop("color", "var(--text-dim)"),
	)

	// Compare verdict — one plain-English sentence above the comparison table. A calm,
	// slightly emphasised lead-in that frames the numbers below it.
	rule(".goal-compare-verdict",
		prop("margin", "0.25rem 0 0.75rem"),
		prop("padding", "0.55rem 0.7rem"),
		prop("border", "1px solid var(--border)"),
		prop("border-radius", "var(--radius-lg)"),
		prop("background", "var(--bg-elev)"),
		prop("color", "var(--text)"),
		prop("font-size", "var(--type-14)"),
		prop("line-height", "1.45"),
	)

	// To-do note: a task's note now sits on its own line below the meta row and clamps to
	// two lines instead of a single-line ellipsis. A long one is a button that expands on
	// click; a short one renders plain (no affordance).
	rule(".todo-note-row",
		prop("margin-top", "0.2rem"),
		prop("max-width", "100%"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.4"),
		prop("color", "var(--text-faint)"),
	)
	rule(".todo-note-row.is-clamp2",
		// Button reset (the expandable note is a <button> for keyboard + click).
		prop("display", "-webkit-box"),
		prop("-webkit-box-orient", "vertical"),
		prop("-webkit-line-clamp", "2"),
		prop("line-clamp", "2"),
		prop("overflow", "hidden"),
		prop("white-space", "normal"),
		prop("text-overflow", "ellipsis"),
		prop("text-align", "left"),
		prop("background", "transparent"),
		prop("border", "0"),
		prop("padding", "0"),
		prop("margin-top", "0.2rem"),
		prop("font", "inherit"),
		prop("font-size", "var(--type-13)"),
		prop("line-height", "1.4"),
		prop("color", "var(--text-faint)"),
		prop("cursor", "pointer"),
	)
	rule(".todo-note-row.is-clamp2:hover, .todo-note-row.is-clamp2:focus-visible",
		prop("color", "var(--text-dim)"),
	)
	// Expanded: drop the clamp and brighten to full foreground for comfortable reading.
	rule(".todo-note-row.is-clamp2.is-open",
		prop("-webkit-line-clamp", "unset"),
		prop("line-clamp", "unset"),
		prop("overflow", "visible"),
		prop("color", "var(--text)"),
	)
}
