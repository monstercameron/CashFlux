// SPDX-License-Identifier: MIT

package styles

// registerRailSearch styles the sidebar's destination filter.
//
// It borrows the shape the budgets filter already established — a relatively
// positioned wrapper, an absolutely placed icon, and left padding on the field to
// clear it — so the two search boxes in the product look like the same control
// rather than two people's idea of one. What differs is density: this one sits in
// a 240px rail between the workspace switcher and the nav, so it is a row shorter
// and its type is a step smaller.
func registerRailSearch() {
	rule(".railsearch",
		position("relative"),
		display("flex"),
		alignItems("center"),
		padding("0.35rem 0.5rem 0.5rem"),
	)
	rule(".railsearch-icon",
		position("absolute"),
		left("1.05rem"),
		color("var(--text-faint)"),
		// The icon is decoration over a field; clicks belong to the input beneath it.
		pointerEvents("none"),
		transition("color var(--motion-micro) var(--ease-exit)"),
	)
	// Borderless at rest, and it earns a surface when you use it.
	//
	// It was a bordered box directly under the workspace control's bordered box —
	// two identical rectangles doing unrelated jobs, which is what made the top of
	// the rail read as a form rather than a menu. A filter is a place to type, not
	// a field to fill in: at rest it is a magnifier and a prompt, and the moment it
	// has focus it becomes a proper input with a ring.
	rule(".railsearch-input",
		width("100%"),
		height("2rem"),
		paddingLeft("1.75rem"),
		paddingRight("1.75rem"),
		border("1px solid transparent"),
		borderRadius("8px"),
		background("transparent"),
		color("var(--text)"),
		fontSize("0.8125rem"),
		transition("background var(--motion-micro) var(--ease-exit), border-color var(--motion-micro) var(--ease-exit)"),
	)
	rule(".railsearch-input::placeholder", color("var(--text-faint)"))
	rule(".railsearch:hover .railsearch-input", background("var(--hover)"))
	rule(".railsearch-input:focus, .railsearch-input:focus-visible",
		background("var(--bg-elev)"),
		borderColor("var(--accent)"),
		outline("none"),
	)
	rule(".railsearch:focus-within .railsearch-icon", color("var(--text)"))
	// The clear button only exists while there is something to clear, so it can sit
	// inside the field's right padding without ever crowding the placeholder.
	rule(".railsearch-clear",
		position("absolute"),
		right("0.85rem"),
		display("grid"),
		placeItems("center"),
		width("1.25rem"),
		height("1.25rem"),
		borderRadius("var(--radius-sm)"),
		color("var(--text-faint)"),
		cursor("pointer"),
	)
	rule(".railsearch-clear:hover", color("var(--text)"))
	rule(".railsearch-empty",
		padding("0.5rem 0.25rem 0.75rem"),
		color("var(--text-dim)"),
		fontSize("0.8125rem"),
		lineHeight("1.45"),
	)
	rule(".railsearch-empty-hint",
		marginTop("0.15rem"),
		color("var(--text-faint)"),
	)
	// The rail collapses to an icon strip; the filter is not rendered there at all,
	// but the rule keeps a stale node from ever showing through mid-transition.
	rule(".rail.collapsed .railsearch", display("none"))
}
